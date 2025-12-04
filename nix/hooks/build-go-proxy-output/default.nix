{ stdenv, fetchers, hooks, go }:
let
  # Rely on a "vendored" version of the mod Go package
  mod = fetchers.fetchModuleProxy {
      goPackagePath = "golang.org/x/mod";
      version = "v0.30.0";
      hash = "sha256-dEjRvA/ak+JgGyfQ3jzMc/uiznogPtqv2j+C6xJASqU=";
    };
in
  stdenv.mkDerivation {
    name = "build-go-proxy-output";
    src = ./.;
    env.GOPROXY = "file://${mod}/cache/download";
    preBuild = ''
      env HOME=$TMPDIR go mod download golang.org/x/mod
    '';
    goBuildPackages = "github.com/adisbladis/gobuild.nix/nix/hooks/build-go-proxy-output";
    nativeBuildInputs = [
      hooks.buildGo
      hooks.installGo
      go
    ];
    meta.mainProgram = "build-go-proxy-output";
  }
