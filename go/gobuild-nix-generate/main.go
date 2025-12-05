package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"

	"golang.org/x/mod/modfile"
	"golang.org/x/sync/errgroup"
)

const SCHEMA_VERSION = 1

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
	Version string   `json:"version"`
	Hash    string   `json:"hash"`
	Require []string `json:"require,omitempty"`
}

// Map goPackagePath -> lock entry
type lockFile struct {
	Schema int                       `json:"schema"`
	Cycles map[string]int            `json:"cycles,omitempty"`
	Locked map[string]*goPackageLock `json:"locked"`
}

func common(directory string) (*lockFile, error) {
	// goModPath := filepath.Join(directory, "go.mod")

	// log.WithFields(log.Fields{
	// 	"modPath": goModPath,
	// }).Info("Parsing go.mod")

	var modDownloads []*goModDownload
	{
		// log.Info("Downloading dependencies")

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

		// log.Info("Done downloading dependencies")
	}

	var lockMux sync.Mutex
	lock := &lockFile{
		Schema: SCHEMA_VERSION,
		Locked: make(map[string]*goPackageLock),
		Cycles: make(map[string]int),
	}

	eg := errgroup.Group{}
	eg.SetLimit(10)
	for _, download := range modDownloads {
		// download := download
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

			cmd := exec.Command(
				// TODO: Embed
				"nix-instantiate", "--expr", "((import <nixpkgs> { }).callPackage /home/adisbladis/sauce/github.com/adisbladis/gobuild.nix/nix/fetchers/default.nix { }).fetchModuleProxy", "--argstr", "goPackagePath", download.Path, "--argstr", "version", download.Version,
			)
			output, err := cmd.Output()
			if err != nil {
				fmt.Println(cmd)
				return fmt.Errorf("cmd.Output() failed with %s\n", err)
			}
			drvPath := strings.TrimSpace(string(output))

			cmd = exec.Command(
				"nix-store", "-r", drvPath,
			)

			stdoutPipe, err := cmd.StderrPipe()
			if err != nil {
				return fmt.Errorf("Error getting StdoutPipe: %w", err)
			}

			err = cmd.Start()
			if err != nil {
				return fmt.Errorf("Error starting command: %w", err)
			}

			var hash string
			scanner := bufio.NewScanner(stdoutPipe)
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

	for i, cycle := range findAllCycles(lock.Locked) {
		for _, depGoPackagePath := range cycle {
			lock.Cycles[depGoPackagePath] = i
		}
	}

	return lock, nil
}

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	lock, err := common(cwd)
	if err != nil {
		panic(err)
	}

	lockJson, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		panic(err)
	}

	err = os.WriteFile("gobuild-nix.lock", lockJson, os.FileMode(0644))
	if err != nil {
		fmt.Printf("Error writing to file: %v\n", err)
		return
	}
}
