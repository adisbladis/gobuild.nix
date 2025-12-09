{
  go,
  newScope,
  lib,
}:
lib.makeScope newScope (
  final:
  let
    inherit (final) callPackage;
  in
  {
    # Tooling
    inherit go;

    # Empty initial list of require's until overriden by generated overlay
    require = [ ];

    goPackages = final;

    gobuild-nix-gocacheprog = callPackage (
      { stdenv, hooks }:
      stdenv.mkDerivation {
        name = "gobuild-nix-gocacheprog";
        src = ../go/gobuild-nix-gocacheprog;
        nativeBuildInputs = [
          go
        ];

        installPhase = ''
          runHook preInstall
          export HOME=$TMPDIR
          env GOMAXPROCS=$NIX_BUILD_CORES GOBIN="$out/bin" go install -v ./...
          runHook postInstall
        '';

        meta.mainProgram = "gobuild-nix-gocacheprog";
      }
    ) { };

    gobuild-nix-tool = callPackage ./hooks/gobuild-nix-tool { };

    fetchers = callPackage ./fetchers { };

    hooks = callPackage ./hooks { };

    # Go standard library.
    "std" = callPackage (
      {
        stdenv,
        hooks,
      }:
      stdenv.mkDerivation {
        inherit (go) pname version;
        dontUnpack = true;

        goBuildPackages = "...";

        nativeBuildInputs = [
          hooks.configureGoCache
          hooks.configureGo
          hooks.buildGo
          hooks.buildGoCacheOutputSetupHook
          go
        ];
      }
    ) { };
  }
)
