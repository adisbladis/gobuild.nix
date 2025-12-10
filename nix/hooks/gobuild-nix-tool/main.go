package main

import (
	"encoding/json"
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

	"github.com/adisbladis/gobuild.nix/nix/hooks/gobuild-nix-tool/modcache"
	"github.com/adisbladis/gobuild.nix/nix/hooks/gobuild-nix-tool/parexec"
)

const PROXY_DIR = ".gobuild-proxy"
const SRC_DIR = "src"
const DUMMY_PACKAGE_PATH = "gobuild.nix/build"
const MODCACHE_DIR = "go/pkg/mod"

var nixBuildCores int = 4 // Default value, set in main()

func isGoProxyDir(dir string) bool {
	_, err := os.Lstat(filepath.Join(dir, "go.mod"))
	if err == nil {
		return false
	}
	_, err = os.Lstat(filepath.Join(dir, "cache", "download"))
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

// Find & read all mod files from $proxy/cache/download
func loadGoproxyModfiles(goProxyDir string) (map[string]*modfile.File, error) {
	modfiles := map[string]*modfile.File{}

	executor := parexec.NewParExecutor(nixBuildCores)
	var mux sync.Mutex

	proxyMods, err := modcache.FindGoproxyMods(goProxyDir)
	if err != nil {
		return nil, err
	}

	for _, modFilePath := range proxyMods {
		executor.Go(func() error {

			contents, err := os.ReadFile(modFilePath)
			if err != nil {
				return err
			}

			mod, err := modfile.Parse(modFilePath, contents, nil)
			if err != nil {
				return err
			}

			mux.Lock()
			modfiles[modFilePath] = mod
			mux.Unlock()
			return nil
		})
	}

	return modfiles, executor.Wait()
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

// Find & read all proxy mod files from dependencies we're directly building _only_
func loadSrcProxiesModfiles() (map[string]*modfile.File, error) {
	modfiles := map[string]*modfile.File{}

	executor := parexec.NewParExecutor(nixBuildCores)
	var mux sync.Mutex

	srcProxies := getSrcProxies()
	for _, downloadDir := range srcProxies {
		executor.Go(func() error {
			proxyMods, err := modcache.FindGoproxyMods(downloadDir)
			if err != nil {
				return err
			}

			for _, modFilePath := range proxyMods {
				contents, err := os.ReadFile(modFilePath)
				if err != nil {
					return err
				}

				mod, err := modfile.Parse(modFilePath, contents, nil)
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

	srcs, err := getProxySrcs()
	if err != nil {
		return err
	}

	// Any directory containing a cache/download directory is a Go modules directory
	var goModCacheDirs []string
	{
		var dirMux sync.Mutex
		executor := parexec.NewParExecutor(nixBuildCores)
		for _, srcDir := range srcs {
			executor.Go(func() error {
				if fileExists(filepath.Join(srcDir, "cache", "download")) {
					dirMux.Lock()
					goModCacheDirs = append(goModCacheDirs, srcDir)
					dirMux.Unlock()
				}
				return nil
			})
		}
		err = executor.Wait()
		if err != nil {
			return err
		}
	}

	// Create nix-support dir
	err = os.MkdirAll(nixSupport, 0755)
	if err != nil {
		return err
	}

	// Write output setup hook
	file, err := os.OpenFile(filepath.Join(nixSupport, "setup-hook"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	for _, goModCacheDir := range goModCacheDirs {
		_, err = fmt.Fprintf(file, "addToSearchPath NIX_GOBUILD_MODCACHE '%s'", goModCacheDir)
		if err != nil {
			return err
		}

		_, err = file.Write([]byte("\n"))
		if err != nil {
			return err
		}
	}

	return nil
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

	// Unpack module proxy dirs into ~/go
	// While I'd _love_ a flat layout like lndir we can't have that with Go because
	// the compiler doesn't traverse into directory symlinks
	goModVersions, err := modcache.LinkRecursive(filepath.Join("go", "pkg", "mod"), modcacheDirs, nixBuildCores)
	if err != nil {
		return err
	}

	// Unpack "local" package or create an intermediate package
	{
		// Unpack a local `src` package if it's provided.
		// If we're only building Go proxy sources create a dummy intermediate module that we can build inside.
		srcDir, ok := os.LookupEnv("src")
		if ok && fileExists(filepath.Join(srcDir, "go.mod")) || fileExists(filepath.Join(srcDir, "go.work")) {
			err = copyDir(srcDir, SRC_DIR, goModVersions)
			if err != nil {
				return err
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

			for path, version := range goModVersions {
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
		}

		{
			args := append([]string{"mod", "download"}, slices.Collect(maps.Keys(goModVersions))...)
			cmd := exec.Command("go", args...)
			cmd.Dir = SRC_DIR
			cmd.Stderr = os.Stderr
			cmd.Stdout = os.Stdout
			err = cmd.Run()
			if err != nil {
				return err
			}
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
