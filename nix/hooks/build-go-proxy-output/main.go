package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/semver"
)

// Find all .mod files in a go proxy directory
func findGoMods(goProxyDir string) ([]string, error) {
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
				recurse(filepath.Join(rootDir, file.Name()))
			}
		}

		return nil
	}

	return goMods, recurse(goProxyDir)
}

func copyFile(srcPath, dstPath string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("error reading %s: %v", srcPath, err)
	}
	defer src.Close()

	dst, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("error creating %s: %v", dstPath, err)
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	if err != nil {
		return fmt.Errorf("error copying %s to %s: %v", srcPath, dstPath, err)
	}
	return nil
}

func readModFile(filePath string) (*modfile.File, error) {
	contents, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %v", filePath, err)
	}

	return modfile.Parse(filePath, contents, nil)
}

func main() {
	// Get output directory
	out, ok := os.LookupEnv("out")
	if !ok {
		panic("No 'out' environment variable set")
	}
	out = filepath.Join(out, "goproxy")


	// Get source inputs
	var srcs []string
	{
		src, ok := os.LookupEnv("src")
		if ok {
			srcs = append(srcs, src)
		}

		srcsString, ok := os.LookupEnv("srcs")
		if ok {
			srcs = append(srcs, strings.Fields(srcsString)...)
		}

		if len(srcs) == 0 {
			panic("Attempted to build Go proxy with no sources")
		}
	}

	// List all inputs .mod files
	var inputMods []string
	{
		nixGobuildProxy, ok := os.LookupEnv("NIX_GOBUILD_PROXY")
		if ok {
			for _, proxyDir := range strings.Split(nixGobuildProxy, ":") {
				mods, err := findGoMods(proxyDir) //findGoMods(filepath.Join(proxyDir, "cache", "download"))
				if err != nil {
					panic(err)
				}
				inputMods = append(inputMods, mods...)
			}
		}
	}

	// Collect all modfiles of dependencies
	fileGoMods := map[string]*modfile.File{}
	for _, filePath := range inputMods {
		contents, err := os.ReadFile(filePath)
		if err != nil {
			panic(err)
		}

		// Parse go.mod
		mod, err := modfile.Parse(filePath, contents, nil)
		if err != nil {
			panic(err)
		}

		fileGoMods[filePath] = mod
	}

	// .mod files in goproxy output
	outputMods := []string{}

	// Symlink the structure of the go proxy into the output directory
	// reading all .mod files that we will mutate & write out later
	var writeProxyDir func(string, string) error
	writeProxyDir = func(input, output string) error {
		entries, err := os.ReadDir(input)
		if err != nil {
			panic(err)
		}

		for _, entry := range entries {
			src := filepath.Join(input, entry.Name())
			dst := filepath.Join(output, entry.Name())

			stat, err := os.Stat(src)
			if err != nil {
				return err
			}

			if stat.IsDir() {
				err = os.MkdirAll(dst, stat.Mode().Perm()|0222)
				if err != nil {
					return err
				}

				err = writeProxyDir(src, dst)
				if err != nil {
					return err
				}
			} else {
				if strings.HasSuffix(entry.Name(), ".mod") {
					// err = copyFile(src, dst)
					// if err != nil {
					// 	return err
					// }

					mod, err := readModFile(src)
					if err != nil {
						return err
					}

					outputMods = append(outputMods, dst)
					fileGoMods[dst] = mod
				} else {
					os.Symlink(src, dst)
				}
			}
		}

		return nil
	}

	// Link input sources into output goproxy
	// Any .mod files are copied for later mutation
	for _, src := range srcs {
		err := writeProxyDir(filepath.Join(src, "cache", "download"), filepath.Join(out, "cache", "download"))
		if err != nil {
			panic(err)
		}
	}

	// Group them by their Go package paths, sorting them and picking the
	// latest version that was in the dependency closure to propagate.
	goModVersions := map[string]string{}
	{
		type depGoMod struct {
			version string
			// filePath string
			mod *modfile.File
		}

		depGoModsTemp := map[string][]*depGoMod{} // Temporary array variant of depGoMods
		for filePath, mod := range fileGoMods {
			// If we're looking at a file literally called go.mod we have no clue about it's version
			basePath := filepath.Base(filePath)
			if basePath == "go.mod" {
				continue
			}

			depGoModsTemp[mod.Module.Mod.Path] = append(depGoModsTemp[mod.Module.Mod.Path], &depGoMod{
				// version: nil,
				// filePath: filePath,
				version: strings.TrimSuffix(basePath, ".mod"),
				mod: mod,
			})
		}

		for goPackagePath, mods := range depGoModsTemp {
			slices.SortFunc(mods, func(a, b *depGoMod) int {
				return semver.Compare(a.version, b.version)
			})
			goModVersions[goPackagePath] = mods[len(mods)-1].version
		}
	}

	// Write out all .mod files into output while replacing all output
	// go.mod references with the ones in our closure
	for _, filePath := range outputMods {
		mod, ok := fileGoMods[filePath]
		if !ok {
			panic(fmt.Errorf("Value for file path %s not found", filePath))
		}

		for _, require := range mod.Require {
			localVersion, ok := goModVersions[require.Mod.Path]
			if ok {
				require.Mod.Version = localVersion
			}
		}

		mod.SetRequire(mod.Require) // Might seem redundant but it updates the internal Syntax tree

		newMod, err := mod.Format()
		if err != nil {
			panic(err)
		}

		err = os.WriteFile(filePath, newMod, 0666)
		if err != nil {
			panic(err)
		}
	}
}
