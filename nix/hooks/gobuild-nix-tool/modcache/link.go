package modcache

import (
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"golang.org/x/mod/modfile"

	"github.com/adisbladis/gobuild.nix/nix/hooks/gobuild-nix-tool/parexec"
)

func replaceDependencyVersion(modfile *modfile.File, replacements map[string]string) {
	for _, require := range modfile.Require {
		localVersion, ok := replacements[require.Mod.Path]
		if ok {
			require.Mod.Version = localVersion
		}
	}
	modfile.SetRequire(modfile.Require) // Might seem redundant but it updates the internal Syntax tree
}

func LinkRecursive(target string, sources []string, workers int) (map[string]string, error) {
	walker := newDirWalker(sources, workers)
	files, err := walker.Walk()
	if err != nil {
		return nil, err
	}

	// Create directories
	var dirs []string
	{
		executor := parexec.NewParExecutor(workers)
		for filename := range files {
			dirs = append(dirs, filepath.Dir(filename))
		}
		slices.Sort(dirs)
		for _, dir := range slices.Compact(dirs) {
			executor.Go(func() error {
				return os.MkdirAll(filepath.Join(target, dir), 0755)
			})
		}
		if err = executor.Wait(); err != nil {
			return nil, err
		}
	}

	// Create symlinks & read go.mod files
	goModVersions := map[string]string{}
	goModFiles := make(map[string]*modfile.File)
	{
		var goModsMux sync.Mutex
		type depGoMod struct {
			version string
			mod     *modfile.File
		}

		readMod := func(filename string, src string) error {
			contents, err := os.ReadFile(src)
			if err != nil {
				return err
			}

			mod, err := modfile.Parse(src, contents, nil)
			if err != nil {
				return err
			}

			goModsMux.Lock()

			goModFiles[filename] = mod
			// We can only read version numbers from GOPROXY mod file names
			base := filepath.Base(filename)
			if strings.HasPrefix(filename, "cache/download") && base != "go.mod" {
				goModVersions[mod.Module.Mod.Path] = strings.TrimSuffix(base, ".mod")
			}

			goModsMux.Unlock()

			return nil
		}

		executor := parexec.NewParExecutor(workers)
		for filename, src := range files {
			if strings.HasPrefix(filename, "cache/download") && filepath.Ext(filename) == ".mod" {
				executor.Go(func() error {
					return readMod(filename, src)
				})
			} else {
				switch filepath.Base(filename) {
				// Read go.mod files
				case "go.mod":
					executor.Go(func() error {
						return readMod(filename, src)
					})
				case "go.sum":
					continue
				default:
					executor.Go(func() error {
						dstFile := filepath.Join(target, filename)

						// Non Go files may be embedded using go:embed and cannot be a symlink
						if strings.HasSuffix(filename, ".go") {
							return os.Symlink(src, dstFile)
						} else {
							srcFile, err := os.Open(src)
							if err != nil {
								return err
							}
							defer srcFile.Close()

							dstFile, err := os.Create(dstFile)
							if err != nil {
								return err
							}
							defer dstFile.Close()

							_, err = io.Copy(dstFile, srcFile)
							return err
						}
					})
				}
			}
		}

		if err = executor.Wait(); err != nil {
			return nil, err
		}
	}

	for _, mod := range goModFiles {
		replaceDependencyVersion(mod, goModVersions)
	}

	{
		executor := parexec.NewParExecutor(workers)

		for relPath, mod := range goModFiles {
			executor.Go(func() error {
				contents, err := mod.Format()
				if err != nil {
					return err
				}

				err = os.WriteFile(filepath.Join(target, relPath), contents, 0644)
				if err != nil {
					return err
				}

				return nil
			})
		}

		err := executor.Wait()
		if err != nil {
			return nil, err
		}
	}

	return goModVersions, nil
}
