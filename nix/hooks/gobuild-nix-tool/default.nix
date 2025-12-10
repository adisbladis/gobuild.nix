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
    name = "gobuild-nix-tool";
    src = ./.;
    env = {
      GOSUMDB = "off";
    };
    preConfigure = ''
      rm go.sum
      sed -i s/'^go .*$'/'go ${go.version}'/ go.mod
      export HOME=$TMPDIR
      mkdir -p $HOME/go/pkg/mod
      ln -s ${mod}/* $HOME/go/pkg/mod/
      export GOPROXY=file://$HOME/go/pkg/mod/cache/download
      go get -v golang.org/x/mod
    '';
    installPhase = ''
      runHook preInstall
      env GOMAXPROCS=$NIX_BUILD_CORES GOBIN="$out/bin" go install -v ./...
      runHook postInstall
    '';

    nativeBuildInputs = [
      go
    ];
    meta.mainProgram = "gobuild-nix-tool";
  }
