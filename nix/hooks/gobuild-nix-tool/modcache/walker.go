package modcache

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type dirWalker struct {
	sourceDirs []string
	fileMap    map[string]string
	mu         sync.RWMutex
	sem        chan struct{}
	wg         sync.WaitGroup
	errChan    chan error
	errOnce    sync.Once
	firstError error
}

func newDirWalker(sourceDirs []string, maxWorkers int) *dirWalker {
	return &dirWalker{
		fileMap:    make(map[string]string),
		sem:        make(chan struct{}, maxWorkers),
		errChan:    make(chan error, 100),
		sourceDirs: sourceDirs,
	}
}

func (dw *dirWalker) walkRecursive(currentPath, basePath string) {
	dw.wg.Go(func() {
		dw.sem <- struct{}{}
		defer func() { <-dw.sem }()

		entries, err := os.ReadDir(currentPath)
		if err != nil {
			dw.errChan <- fmt.Errorf("error reading directory %s: %w", currentPath, err)
			return
		}

		var dirs []string

		dw.mu.Lock()
		for _, entry := range entries {
			fullPath := filepath.Join(currentPath, entry.Name())

			if entry.IsDir() {
				dirs = append(dirs, fullPath)
			} else {
				relPath, err := filepath.Rel(basePath, fullPath)
				if err != nil {
					dw.errChan <- fmt.Errorf("failed to get relative path for %s: %w", fullPath, err)
					continue
				}

				dw.fileMap[relPath] = fullPath
			}
		}
		dw.mu.Unlock()

		for _, fullPath := range dirs {
			dw.walkRecursive(fullPath, basePath)
		}
	})
}

func (dw *dirWalker) Walk() (map[string]string, error) {
	go func() {
		for err := range dw.errChan {
			dw.errOnce.Do(func() {
				dw.firstError = err
			})
		}
	}()

	for _, dir := range dw.sourceDirs {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			return nil, fmt.Errorf("failed to get absolute path for %s: %w", dir, err)
		}

		info, err := os.Stat(absDir)
		if err != nil {
			return nil, fmt.Errorf("failed to stat %s: %w", absDir, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("%s is not a directory", absDir)
		}

		dw.walkRecursive(absDir, absDir)
	}

	dw.wg.Wait()
	close(dw.errChan)

	return dw.fileMap, dw.firstError
}
