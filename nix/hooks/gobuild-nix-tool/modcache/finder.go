package modcache

import (
	"os"
	"path/filepath"
)

func FindGoproxyMods(goProxyDir string) ([]string, error) {
	var goMods []string

	var recurse func(string) error
	recurse = func(rootDir string) error {
		files, err := os.ReadDir(rootDir)
		if err != nil {
			return err
		}

		for _, file := range files {
			switch file.Name() {
			case "@v":
				matches, err := filepath.Glob(filepath.Join(rootDir, file.Name(), "*.mod"))
				if err != nil {
					return err
				}

				for _, match := range matches {
					goMods = append(goMods, match)
				}
			default:
				path := filepath.Join(rootDir, file.Name())
				info, err := os.Lstat(path)
				if err != nil {
					return err
				}

				if info.IsDir() {
					err = recurse(path)
					if err != nil {
						return err
					}
				}
			}
		}

		return nil
	}

	return goMods, recurse(goProxyDir)
}
