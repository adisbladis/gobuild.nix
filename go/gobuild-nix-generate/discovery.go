package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
	"golang.org/x/sync/errgroup"
)

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

func downloadModules(directory string, packages []string) ([]*goModDownload, error) {
	var downloads []*goModDownload

	cmd := exec.Command("go", append([]string{"mod", "download", "--json"}, packages...)...)
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
		} else {
			downloads = append(downloads, dl)
		}
	}

	return downloads, nil
}

func downloadModule(directory string, packagePath string, version string) (*goModDownload, error) {
	log.Printf("Downloading %s@%s", packagePath, version)

	cmd := exec.Command("go", "mod", "download", "--json", fmt.Sprintf("%s@%s", packagePath, version))
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
		} else {
			return dl, err
		}
	}

	return nil, fmt.Errorf("error downloading %s@%s: no module download returned", packagePath, version)
}

func discoverDependencies(directory string, workers int, sumVersions map[string]string) ([]*goModDownload, error) {
	var downloadMod func(context.Context, module.Version) error
	var discoverMod func(module.Version)
	var wg sync.WaitGroup

	downloads := map[string]*goModDownload{}
	var downloadsMu sync.RWMutex

	eg := errgroup.Group{}
	eg.SetLimit(workers)

	discoveredVersions := map[string]string{}
	var discoveredMu sync.RWMutex
	discoverSem := make(chan struct{}, workers)
	errChan := make(chan error, workers)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	discoverMod = func(mod module.Version) {
		if mod.Path[0] == '.' { // Disregard local replacements
			return
		}

		wg.Add(1)

		doWork := func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case discoverSem <- struct{}{}:
			}

			defer func() {
				<-discoverSem
				wg.Done()
			}()

			// If package was found through go.sum it's always a fixed version
			if prevVersion, ok := sumVersions[mod.Path]; ok {
				discoveredMu.Lock()
				discoveredVersions[mod.Path] = prevVersion
				discoveredMu.Unlock()

				return downloadMod(ctx, module.Version{
					Path:    mod.Path,
					Version: prevVersion,
				})
			}

			discoveredMu.RLock()
			prevVersion, ok := discoveredVersions[mod.Path]
			discoveredMu.RUnlock()

			if ok {
				if semver.Compare(mod.Version, prevVersion) == 1 {
					discoveredMu.Lock()
					discoveredVersions[mod.Path] = mod.Version
					discoveredMu.Unlock()
					return downloadMod(ctx, mod)
				} else {
					return nil
				}
			} else {
				discoveredMu.Lock()
				discoveredVersions[mod.Path] = mod.Version
				discoveredMu.Unlock()
				return downloadMod(ctx, mod)
			}
		}

		go func() {
			if err := doWork(); err != nil {
				select {
				case errChan <- err:
				default:
					// Channel full, error already reported
				}
			}
		}()
	}

	downloadMod = func(ctx context.Context, goModule module.Version) error {
		// Check for cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		key := fmt.Sprintf("%s@%s", goModule.Path, goModule.Version)

		// Check if already being handled
		downloadsMu.Lock()
		_, ok := downloads[key]

		if ok {
			downloadsMu.Unlock()
			return nil
		}

		// Set a nil value to indicate it is _being_ handled, but not handled yet
		_, exists := downloads[key]
		if exists {
			downloadsMu.Unlock()
			return nil
		}
		downloads[key] = nil
		downloadsMu.Unlock()

		download, err := downloadModule(directory, goModule.Path, goModule.Version)
		if err != nil {
			return fmt.Errorf("error downloading module %s@%s: %w", goModule.Path, goModule.Version, err)
		}

		contents, err := os.ReadFile(download.GoMod)
		if err != nil {
			return fmt.Errorf("error reading %s: %v", download.GoMod, err)
		}

		mod, err := modfile.Parse(download.GoMod, contents, nil)
		if err != nil {
			return fmt.Errorf("error parsing %s: %v", download.GoMod, err)
		}

	requireLoop:
		for _, require := range mod.Require {
			// Check for cancellation in loop
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			for _, replacement := range mod.Replace {
				if require.Mod.Path == replacement.Old.Path {
					discoverMod(replacement.New)
					continue requireLoop
				}
			}

			discoverMod(require.Mod)
		}

		downloadsMu.Lock()
		downloads[key] = download
		downloadsMu.Unlock()

		return nil
	}

	// Start a goroutine to monitor for errors and cancel on first error
	done := make(chan struct{})
	var firstErr error
	go func() {
		for err := range errChan {
			if firstErr == nil {
				firstErr = err
				cancel() // Cancel all ongoing work on first error
			}
		}
		close(done)
	}()

	for packagePath, version := range sumVersions {
		discoverMod(module.Version{
			Path:    packagePath,
			Version: version,
		})
	}

	wg.Wait()
	close(errChan)
	<-done

	if firstErr != nil {
		return nil, firstErr
	}

	modDownloads := make([]*goModDownload, len(discoveredVersions))
	i := 0
	for packagePath, version := range discoveredVersions {
		modDownloads[i] = downloads[fmt.Sprintf("%s@%s", packagePath, version)]
		i++
	}

	slices.SortFunc(modDownloads, func(a, b *goModDownload) int {
		return strings.Compare(a.Path, b.Path)
	})

	return modDownloads, nil
}
