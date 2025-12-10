package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/mod/modfile"

	"github.com/adisbladis/gobuild.nix/nix/hooks/gobuild-nix-tool/parexec"
)

const PROXY_DIR = ".gobuild-proxy"
const SRC_DIR = "src"
const DUMMY_PACKAGE_PATH = "gobuild.nix/build"
const MODCACHE_DIR = "go/pkg/mod"

var nixBuildCores int = 4 // Default value, set in main()

func isGoProxyDir(dir string) bool {
	_, err := os.Lstat(filepath.Join(dir, "cache", "download"))
	return err == nil
}

// Get source input directories for
func getProxySrcs() ([]string, error) {
	var srcs []string

	src, ok := os.LookupEnv("src")
	if ok && isGoProxyDir(src) {
		srcs = append(srcs, src)
	}

	srcsString, ok := os.LookupEnv("srcs")
	if ok {
		if len(srcs) >= 1 {
			return nil, fmt.Errorf("environment variables 'src' & 'srcs' are mutually exclusive")
		}

		for srcDir := range strings.FieldsSeq(srcsString) {
			if !isGoProxyDir(srcDir) {
				return nil, fmt.Errorf("srcs directory '%s' is not a Go module cache directory, bailing out", srcDir)
			}

			srcs = append(srcs, srcDir)
		}
	}

	return srcs, nil
}

func findGoproxyMods(goProxyDir string) ([]string, error) {
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

// Get gobuild.nix GOMODCACHE directories
func getModCaches() []string {
	var modCaches []string

	nixGobuildProxy, ok := os.LookupEnv("NIX_GOBUILD_MODCACHE")
	if !ok {
		return modCaches
	}

	for inputDir := range strings.SplitSeq(nixGobuildProxy, ":") {
		if inputDir != "" {
			modCaches = append(modCaches, inputDir)
		}
	}

	return modCaches
}

// Get source inputs
func getSrcProxies() []string {
	var srcs []string

	src, ok := os.LookupEnv("src")
	if ok && isGoProxyDir(src) {
		srcs = append(srcs, filepath.Join(src, "cache", "download"))
	}

	srcsString, ok := os.LookupEnv("srcs")
	if ok {
		for srcDir := range strings.FieldsSeq(srcsString) {
			if isGoProxyDir(srcDir) {
				srcs = append(srcs, filepath.Join(srcDir, "cache", "download"))
			}
		}
	}

	return srcs
}

func replaceDependencyVersion(modfile *modfile.File, replacements map[string]string) {
	for _, require := range modfile.Require {
		localVersion, ok := replacements[require.Mod.Path]
		if ok {
			require.Mod.Version = localVersion
		}
	}
	modfile.SetRequire(modfile.Require) // Might seem redundant but it updates the internal Syntax tree
}

// Build Go packages without treating build failures as a hard fail.
// We're building to optimise the number of cache hits.
func buildGoPackages(buildFlags []string, goPackagePaths []string) {
	args := []string{"build", "-v"}
	args = append(args, buildFlags...)
	args = append(args, goPackagePaths...)

	cmd := exec.Command("go", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	env := os.Environ()
	env = append(env, fmt.Sprintf("GOMAXPROCS=%d", nixBuildCores))
	cmd.Env = env

	// Run command and retry each half separately
	if cmd.Run() != nil && len(goPackagePaths) > 1 {
		split := len(goPackagePaths) / 2
		// Split array down the middle and retry each half until completion
		buildGoPackages(buildFlags, goPackagePaths[:split])
		buildGoPackages(buildFlags, goPackagePaths[split:])
	}
}

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

func readMod(path string) (*modfile.File, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	mod, err := modfile.Parse(path, contents, nil)
	if err != nil {
		return nil, err
	}

	return mod, nil
}

// Find & read all proxy mod files from dependencies we're directly building _only_
func loadSrcProxiesModfiles() (map[string]*modfile.File, error) {
	modfiles := map[string]*modfile.File{}

	executor := parexec.NewParExecutor(nixBuildCores)
	var mux sync.Mutex

	srcProxies := getSrcProxies()
	for _, downloadDir := range srcProxies {
		executor.Go(func() error {
			proxyMods, err := findGoproxyMods(downloadDir)
			if err != nil {
				return err
			}

			for _, modFilePath := range proxyMods {
				mod, err := readMod(modFilePath)
				if err != nil {
					return err
				}

				mux.Lock()
				modfiles[modFilePath] = mod
				mux.Unlock()
			}

			return nil
		})
	}

	return modfiles, executor.Wait()
}

func listGoPackages(envvar string) ([]string, error) {
	type goListPackage struct {
		ImportPath string
		GoFiles    []string
	}

	// All packages currently being built
	goPackagePaths := map[string]struct{}{}

	// Recursively list packages
	var listPackages []string
	{
		goPackagesString, ok := os.LookupEnv(envvar)
		if ok {
			listPackages = strings.Fields(goPackagesString)
		} else {
			proxyMods, err := loadSrcProxiesModfiles()
			if err != nil {
				return nil, err
			}

			// Tack on /... to all packages for recursive listing
			listPackages = make([]string, len(proxyMods)+1)
			{
				i := 0
				for _, mod := range proxyMods {
					goPackagePaths[mod.Module.Mod.Path] = struct{}{}
					listPackages[i] = mod.Module.Mod.Path + "/..."
					i++
				}

				// Take a local source package into account
				listPackages[i] = "./..."
			}

		}
	}

	{
		args := append([]string{"list", "-e", "-json"}, listPackages...)
		{
			cmd := exec.Command("go", args...)

			stdout, err := cmd.StdoutPipe()
			if err != nil {
				return nil, err
			}

			if err := cmd.Start(); err != nil {
				return nil, err
			}

			decoder := json.NewDecoder(stdout)
			for {
				var pkg goListPackage
				if err := decoder.Decode(&pkg); err != nil {
					// io.EOF is the expected error when the stream ends
					if err.Error() == "EOF" {
						break
					}
					return nil, err
				}

				if pkg.GoFiles != nil {
					goPackagePaths[pkg.ImportPath] = struct{}{}
				}
			}

			if len(goPackagePaths) == 0 {
				return nil, fmt.Errorf("Found no Go packages while listing")
			}
		}
	}

	packagePaths := slices.Collect(maps.Keys(goPackagePaths))
	slices.Sort(packagePaths)

	return packagePaths, nil
}

func buildGoCmd() error {
	var goPackagePaths []string

	goPackagePaths, err := listGoPackages("goBuildPackages")
	if err != nil {
		return err
	}

	var buildFlags []string
	value, ok := os.LookupEnv("goBuildFlags")
	if ok {
		buildFlags = strings.Fields(value)
	}

	buildGoPackages(buildFlags, goPackagePaths)

	return nil
}

func installGoCmd() error {
	out, ok := os.LookupEnv("out")
	if !ok {
		return fmt.Errorf("No 'out' environment variable set")
	}

	goPackagePaths, err := listGoPackages("goInstallPackages")
	if err != nil {
		return err
	}

	var installFlags []string
	value, ok := os.LookupEnv("goInstalllags")
	if ok {
		installFlags = strings.Fields(value)
	}

	args := append([]string{"install", "-v"}, installFlags...)
	args = append(args, goPackagePaths...)

	cmd := exec.Command("go", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	env := os.Environ()
	env = append(env, fmt.Sprintf("GOMAXPROCS=%d", nixBuildCores))
	env = append(env, fmt.Sprintf("GOBIN=%s", filepath.Join(out, "bin")))
	cmd.Env = env

	return cmd.Run()
}

func buildModCacheOutputSetupHook() error {
	out, ok := os.LookupEnv("out")
	if !ok {
		return fmt.Errorf("No 'out' environment variable set")
	}
	nixSupport := filepath.Join(out, "nix-support")
	modCacheDir := filepath.Join(nixSupport, "gobuild-nix", "mod")

	srcs, err := getProxySrcs()
	if err != nil {
		return err
	}

	moduleVersions, err := discoverModVersionsFromDirs(append(srcs, getModCaches()...), nixBuildCores)
	if err != nil {
		return fmt.Errorf("Error loading module versions: %w", err)
	}

	executor := parexec.NewParExecutor(nixBuildCores)
	for _, srcDir := range srcs {
		executor.Go(func() error {
			err := copyDir(srcDir, modCacheDir, "*.mod", moduleVersions)
			if err != nil {
				return fmt.Errorf("error copying %s to %s: %v", srcDir, modCacheDir, err)
			}
			return nil
		})
	}
	if err = executor.Wait(); err != nil {
		return err
	}

	// Create nix-support/gobuild-nix dir
	if err = os.MkdirAll(modCacheDir, 0755); err != nil {
		return err
	}

	// Write output setup hook
	file, err := os.OpenFile(filepath.Join(nixSupport, "setup-hook"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = fmt.Fprintf(file, "addToSearchPath NIX_GOBUILD_MODCACHE '%s'", modCacheDir)
	if err != nil {
		return err
	}

	_, err = file.Write([]byte("\n"))
	if err != nil {
		return err
	}

	return nil
}

func linkRecursiveFlat(dst string, sources []string) error {
	type foundFile struct {
		path  string
		isDir bool
	}

	var recurse func(string, []string) error
	recurse = func(target string, inputs []string) error {
		if err := os.MkdirAll(target, 0755); err != nil {
			return fmt.Errorf("error creating %s: %w", target, err)
		}

		foundFiles := map[string][]*foundFile{}
		for _, input := range inputs {
			entries, err := os.ReadDir(input)
			if err != nil {
				return fmt.Errorf("error listing %s: %w", input, err)
			}

			for _, entry := range entries {
				name := entry.Name()
				foundFiles[name] = append(foundFiles[name], &foundFile{
					path:  filepath.Join(input, name),
					isDir: entry.IsDir(),
				})
			}
		}

		for name, files := range foundFiles {
			if len(files) > 1 {
				if files[0].isDir {
					nonDirs := []*foundFile{}
					for _, file := range files {
						if !file.isDir {
							nonDirs = append(nonDirs, file)
						}
					}

					if len(nonDirs) > 0 {
						errorMessage := fmt.Sprintf("Found both directory & non directory sources for %s:\n", filepath.Join(target, name))
						for _, file := range files {
							errorMessage += fmt.Sprintf("- %s (%t)\n", file.path, file.isDir)
						}
						return errors.New(errorMessage)
					} else {
						childInputs := make([]string, len(files))
						for i, file := range files {
							childInputs[i] = file.path
						}
						if err := recurse(filepath.Join(target, name), childInputs); err != nil {
							return err
						}
						continue
					}
				}
			}

			from := filepath.Join(files[0].path)
			to := filepath.Join(target, name)
			if err := os.Symlink(from, to); err != nil {
				return fmt.Errorf("error symlinking %s -> %s: %w", from, to, err)
			}
		}

		return nil
	}

	return recurse(dst, sources)
}

func unpackGo() error {
	// Go mod caches from currently built derivation
	proxySrcs, err := getProxySrcs()
	if err != nil {
		return err
	}

	// Go mod caches from other builds
	modCaches := getModCaches()

	// Combined list of all mod caches
	modcacheDirs := append(proxySrcs, modCaches...)

	moduleVersions, err := discoverModVersionsFromDirs(modcacheDirs, nixBuildCores)
	if err != nil {
		return fmt.Errorf("Error loading module versions: %w", err)
	}

	// Symlink modcache dirs into ~/go
	err = linkRecursiveFlat(filepath.Join("go", "pkg", "mod"), modcacheDirs)
	if err != nil {
		return fmt.Errorf("error symlinking sources: %w", err)
	}

	// Unpack "local" package or create an intermediate package
	{
		// Unpack a local `src` package if it's provided.
		// If we're only building Go proxy sources create a dummy intermediate module that we can build inside.
		srcDir, ok := os.LookupEnv("src")
		if ok && fileExists(filepath.Join(srcDir, "go.mod")) || fileExists(filepath.Join(srcDir, "go.work")) {
			err = copyDir(srcDir, SRC_DIR, "go.mod", moduleVersions)
			if err != nil {
				return err
			}

			// Go mod download local dependencies only if we're in a local tree
			modPath := filepath.Join(SRC_DIR, "go.mod")
			if fileExists(modPath) {
				mod, err := readMod(modPath)
				if err != nil {
					return err
				}

				require := make([]string, len(mod.Require))
				for i, req := range mod.Require {
					require[i] = req.Mod.Path
				}

				cmd := exec.Command("go", append([]string{"mod", "download"}, require...)...)
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				cmd.Dir = SRC_DIR
				if err = cmd.Run(); err != nil {
					return err
				}
			} else {
				panic("TODO: Recurse into local go.work")
			}
		} else { // No local package, create dummy package
			err = os.Mkdir(SRC_DIR, 0777)
			if err != nil {
				return err
			}

			mod := &modfile.File{}

			err := mod.AddModuleStmt(DUMMY_PACKAGE_PATH)
			if err != nil {
				return err
			}

			err = mod.AddGoStmt(strings.TrimPrefix(runtime.Version(), "go"))
			if err != nil {
				return err
			}

			for path, version := range moduleVersions {
				err = mod.AddRequire(path, version)
				if err != nil {
					return err
				}
			}

			contents, err := mod.Format()
			if err != nil {
				return err
			}

			err = os.WriteFile(filepath.Join(SRC_DIR, "go.mod"), contents, 0666)
			if err != nil {
				return err
			}

			var downloadModules func([]string)
			downloadModules = func(goPackages []string) {
				args := append([]string{"mod", "download"}, goPackages...)
				cmd := exec.Command("go", args...)
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				cmd.Dir = SRC_DIR
				err = cmd.Run()

				// Run command and retry each half separately
				if cmd.Run() != nil && len(goPackages) > 1 {
					split := len(goPackages) / 2
					downloadModules(goPackages[:split])
					downloadModules(goPackages[split:])
				}
			}

			downloadModules(slices.Collect(maps.Keys(moduleVersions)))
		}
	}

	return nil
}

func main() {
	var err error

	nixBuildCoresString, ok := os.LookupEnv("NIX_BUILD_CORES")
	if ok {
		i, err := strconv.Atoi(nixBuildCoresString)
		if err == nil {
			nixBuildCores = i
		}
	}

	switch os.Args[1] {
	case "buildGo":
		err = buildGoCmd()
	case "unpackGo":
		err = unpackGo()
	case "installGo":
		err = installGoCmd()
	case "buildModCacheOutputSetupHook":
		err = buildModCacheOutputSetupHook()

	default:
		err = fmt.Errorf("Unknown command: %s", os.Args[1])
	}

	if err != nil {
		panic(err)
	}
}
