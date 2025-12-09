package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"
	"golang.org/x/mod/modfile"
	"golang.org/x/sync/errgroup"

	_ "embed"
)

const SCHEMA_VERSION = 1
const LOCK_FILE = "gobuild-nix.lock"

type goModDownload struct {
	Path     string
	Version  string
	Info     string
	GoMod    string
	Zip      string
	Dir      string
	Sum      string
	GoModSum string
}

type goPackageLock struct {
	Version string   `toml:"version"`
	Hash    string   `toml:"hash"`
	Require []string `toml:"require,omitempty"`
}

// Map goPackagePath -> lock entry
type lockFile struct {
	Schema  int                       `toml:"schema"`
	Cycles  map[string]int            `toml:"cycles,omitempty"`
	Locked  map[string]*goPackageLock `toml:"locked"`
	Require []string                  `toml:"require"`
}

func filter[T any](slice []T, predicate func(T) bool) []T {
	var result []T
	for _, v := range slice {
		if predicate(v) {
			result = append(result, v)
		}
	}
	return result
}

//go:embed fetcher.nix
var fetcherExpr string

func createLock(directory string, workers int, pkgsFlag string, attrFlag string) (*lockFile, error) {
	// If we have a previous lock file re-use hashes instead of re-computing them if the package/version is the same
	prevHashes := make(map[string]string)
	if _, err := os.Stat(filepath.Join(directory, LOCK_FILE)); err == nil {
		contents, err := os.ReadFile(filepath.Join(directory, LOCK_FILE))
		if err != nil {
			return nil, fmt.Errorf("error reading previous lockfile: %w", err)
		}

		prevLock := &lockFile{}
		err = toml.Unmarshal(contents, prevLock)
		if err == nil { // If we're erroring out it's probably a schema change, just consider it a cache miss
			for goPackagePath, locked := range prevLock.Locked {
				prevHashes[fmt.Sprintf("%s@%s", goPackagePath, locked.Version)] = locked.Hash
			}
		}
	}

	var modDownloads []*goModDownload
	{
		log.Println("Downloading dependencies")

		cmd := exec.Command(
			"go", "mod", "download", "--json",
		)
		cmd.Dir = directory
		stdout, err := cmd.Output()
		if err != nil {
			if exiterr, ok := err.(*exec.ExitError); ok {
				return nil, fmt.Errorf("failed to run 'go mod download --json: %s\n%s", exiterr, exiterr.Stderr)
			} else {
				return nil, fmt.Errorf("failed to run 'go mod download --json': %s", err)
			}
		}

		dec := json.NewDecoder(bytes.NewReader(stdout))
		for {
			var dl *goModDownload
			err := dec.Decode(&dl)
			if err == io.EOF {
				break
			}
			modDownloads = append(modDownloads, dl)
		}

		log.Println("Done downloading dependencies")
	}

	var lockMux sync.Mutex
	lock := &lockFile{
		Schema: SCHEMA_VERSION,
		Locked: make(map[string]*goPackageLock),
		Cycles: make(map[string]int),
	}

	expr := fmt.Sprintf("(with import %s { }; callPackage (%s) { go = pkgs.\"%s\"; }).fetchModuleProxy", pkgsFlag, fetcherExpr, attrFlag)

	eg := errgroup.Group{}
	eg.SetLimit(workers)
	for _, download := range modDownloads {
		lock.Require = append(lock.Require, download.Path)

		eg.Go(func() error {
			var require []string
			{
				contents, err := os.ReadFile(download.GoMod)
				if err != nil {
					return err // TODO: Wrap with context
				}
				// Parse go.mod
				mod, err := modfile.Parse(download.GoMod, contents, nil)
				if err != nil {
					return err // TODO: Wrap with context
				}

				// Note: You might be tempted to filter out indirect dependencies
				// but this is not possible because some dependencies may be incorrectly declared
				// as indirect when they are in fact direct.
				for _, modRequire := range mod.Require {
					require = append(require, modRequire.Mod.Path)
				}
			}

			hash, ok := prevHashes[fmt.Sprintf("%s@%s", download.Path, download.Version)]
			if !ok {
				log.Printf("Fetching %s", download.Path)

				cmd := exec.Command(
					"nix-instantiate", "--expr", expr, "--argstr", "goPackagePath", download.Path, "--argstr", "version", download.Version,
				)
				output, err := cmd.Output()
				if err != nil {
					return fmt.Errorf("cmd.Output() failed with %s\n", err)
				}
				drvPath := strings.TrimSpace(string(output))

				cmd = exec.Command(
					"nix-store", "-r", drvPath,
				)

				stderrPipe, err := cmd.StderrPipe()
				if err != nil {
					return fmt.Errorf("Error getting StdoutPipe: %w", err)
				}

				err = cmd.Start()
				if err != nil {
					return fmt.Errorf("Error starting command: %w", err)
				}

				scanner := bufio.NewScanner(stderrPipe)
				{
					// Text finder state
					const (
						Looking       int = iota // Didn't find anything yet
						HashMismatch             // Found hash mismatch
						SpecifiedHash            // Found specified hash
						ActualHash               // Found actual hash
					)

					finderState := Looking

					// Find hash mismatch line
					{
						gotRe := regexp.MustCompile(" +got: +(.+)$")
					Scanner:
						for scanner.Scan() {
							line := scanner.Bytes()
							switch finderState {
							case Looking:
								if bytes.HasPrefix(line, []byte("error: hash mismatch in fixed-output")) {
									finderState = HashMismatch
								}
							case HashMismatch:
								found, err := regexp.Match(" +specified: +.+$", line)
								if err != nil {
									return err
								}

								if found {
									finderState = SpecifiedHash
								}
							case SpecifiedHash:
								match := gotRe.FindSubmatch(line)
								if len(match) == 0 {
									continue
								}

								hash = string(match[1])
							case ActualHash:
								break Scanner
							}
						}
					}
					if finderState != SpecifiedHash {
						return fmt.Errorf("Hash mismatch pattern not found in stream")
					}
				}

				if err := scanner.Err(); err != nil {
					fmt.Println("Error reading from stdout:", err)
				}

				cmd.Wait()
			}

			lockMux.Lock()
			lock.Locked[download.Path] = &goPackageLock{
				Version: download.Version,
				Hash:    hash,
				Require: require,
			}
			lockMux.Unlock()

			return nil
		})
	}

	err := eg.Wait()
	if err != nil {
		return nil, err
	}

	// The require list contains modules that are not
	// in our graph.
	// These are optional dependencies not used by the module we're generating for.
	//
	// Filter out unsatisfied requirements
	for _, locked := range lock.Locked {
		locked.Require = filter(locked.Require, func(requirement string) bool {
			_, ok := lock.Locked[requirement]
			return ok
		})
	}

	for i, cycle := range findAllCycles(lock.Locked) {
		for _, depGoPackagePath := range cycle {
			lock.Cycles[depGoPackagePath] = i
		}
	}

	return lock, nil
}

func main() {
	var pkgsFlag = flag.String("f", "<nixpkgs>", "path to custom nixpkgs used for prefetching")
	var jobsFlag = flag.Int("j", 10, "number of max concurrent prefetching jobs")
	var attrFlag = flag.String("a", "go", "go attribute to use for prefetching")

	flag.Parse()

	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	lock, err := createLock(cwd, *jobsFlag, *pkgsFlag, *attrFlag)
	if err != nil {
		panic(err)
	}

	lockContents, err := toml.Marshal(lock)
	if err != nil {
		panic(err)
	}

	err = os.WriteFile(LOCK_FILE, lockContents, os.FileMode(0644))
	if err != nil {
		fmt.Printf("Error writing to file: %v\n", err)
		return
	}

	log.Printf("Wrote %s", LOCK_FILE)
}
