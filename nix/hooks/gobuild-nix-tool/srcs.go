package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/mod/modfile"
)

func copyDir(src, dst string, modGlob string, replacements map[string]string) error {
	stat, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to stat source: %w", err)
	}

	if !stat.IsDir() {
		return fmt.Errorf("source is not a directory: %s", src)
	}

	sem := make(chan struct{}, nixBuildCores)
	var wg sync.WaitGroup
	errChan := make(chan error, 1)
	var errOnce sync.Once

	err = copyDirRecursive(src, dst, sem, &wg, errChan, &errOnce, modGlob, replacements)
	if err != nil {
		return err
	}

	wg.Wait()
	close(errChan)

	if err := <-errChan; err != nil {
		return err
	}

	return nil
}

func copyFile(src, dst string, modGlob string, replacements map[string]string) error {
	base := filepath.Base(src)
	if base == "go.sum" || base == "go.work.sum" { // Don't copy go.sum, they're not usable for us
		return nil
	}

	stat, err := os.Lstat(src)
	if err != nil {
		return fmt.Errorf("failed to stat source file: %w", err)
	}

	if stat.Mode()&os.ModeSymlink != 0 {
		// linkSrc, err := os.Readlink(src)
		// if err != nil {
		// 	return fmt.Errorf("failed to read link %s: %w", src, err)
		// }

		// err = os.Symlink(linkSrc, dst)
		// if err != nil {
		// 	return fmt.Errorf("failed to symlink %s -> %s: %w", src, dst, err)
		// }
	} else {
		isMod, err := filepath.Match(modGlob, filepath.Base(src))
		if err != nil {
			return err
		}

		if isMod {
			// Replace go.mod versions with build input versions
			contents, err := os.ReadFile(src)
			if err != nil {
				return fmt.Errorf("failed to read %s: %w", src, err)
			}

			mod, err := modfile.Parse(src, contents, nil)
			if err != nil {
				return fmt.Errorf("failed to parse %s: %w", src, err)
			}

			replaceDependencyVersion(mod, replacements)

			contents, err = mod.Format()
			if err != nil {
				return fmt.Errorf("failed to create go.mod: %w", err)
			}

			err = os.WriteFile(dst, contents, 0666)
			if err != nil {
				return fmt.Errorf("failed to write %s: %w", dst, err)
			}
		} else {
			// Otherwise copy the input file
			srcFile, err := os.Open(src)
			if err != nil {
				return fmt.Errorf("failed to open source file: %w", err)
			}
			defer srcFile.Close()

			dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, stat.Mode()|0666)
			if err != nil {
				return fmt.Errorf("failed to create destination file: %w", err)
			}
			defer dstFile.Close()

			_, err = io.Copy(dstFile, srcFile)
			if err != nil {
				return fmt.Errorf("failed to copy file contents: %w", err)
			}
		}
	}

	return nil
}

func copyDirRecursive(src, dst string, sem chan struct{}, wg *sync.WaitGroup, errChan chan error, errOnce *sync.Once, modGlob string, replacements map[string]string) error {
	err := os.MkdirAll(dst, 0777)
	if err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			err = copyDirRecursive(srcPath, dstPath, sem, wg, errChan, errOnce, modGlob, replacements)
			if err != nil {
				return err
			}
		} else {
			wg.Add(1)
			go func(src, dst string) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				if err := copyFile(src, dst, modGlob, replacements); err != nil {
					errOnce.Do(func() {
						errChan <- err
					})
				}
			}(srcPath, dstPath)
		}
	}

	return nil
}
