package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/adisbladis/gobuild.nix/nix/hooks/gobuild-nix-tool/parexec"
)

// The proxy protocol escapes upper case A like !a
var moduleNameEscapeRe = regexp.MustCompile(`!([a-z])`)

func discoverModVersions(root string, versions map[string]string, mux *sync.Mutex) error {
	downloadDir := filepath.Join(root, "cache", "download")

	var recurse func(string) error
	recurse = func(relPath string) error {
		dir := filepath.Join(downloadDir, relPath)

		entries, err := os.ReadDir(dir)
		if err != nil {
			return fmt.Errorf("error reading %s: %w", dir, err)
		}

		for _, entry := range entries {
			name := entry.Name()
			if name == "@v" {
				file := filepath.Join(downloadDir, relPath, "@v", "list")

				// Reached a versioned package
				contents, err := os.ReadFile(file)
				if err != nil {
					return fmt.Errorf("error reading %s: %w", file, err)
				}

				// The version is the first line of the file
				version := string(contents[:bytes.Index(contents, []byte("\n"))])

				packagePath := moduleNameEscapeRe.ReplaceAllStringFunc(relPath, func(match string) string {
					return strings.ToUpper(match[1:])
				})

				mux.Lock()
				versions[packagePath] = version
				mux.Unlock()

				continue
			}

			if entry.IsDir() {
				if err = recurse(filepath.Join(relPath, name)); err != nil {
					return err
				}
			}
		}

		return nil
	}

	return recurse("")
}

func discoverModVersionsFromDirs(dirs []string, workers int) (map[string]string, error) {
	versions := map[string]string{}
	var mux sync.Mutex

	executor := parexec.NewParExecutor(workers)
	for _, dir := range dirs {
		executor.Go(func() error {
			err := discoverModVersions(dir, versions, &mux)
			if err != nil {
				return fmt.Errorf("error discovering modules in %s: %w", dir, err)
			}
			return nil
		})
	}

	return versions, executor.Wait()
}
