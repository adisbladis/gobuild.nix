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

    goPackages = final;

    gobuild-nix-cacher = callPackage (
      { stdenv, hooks }:
      stdenv.mkDerivation {
        name = "gobuild-nix-cacher";
        src = ../gobuild-nix-cacher;
        nativeBuildInputs = [
          hooks.buildGo
          hooks.installGo
        ];
        meta.mainProgram = "gobuild-nix-cacher";
      }
    ) { };

    fetchers = callPackage ./fetchers { };

    hooks = callPackage ./hooks { };

    # Go standard library.
    # This needs to be built in a bit of a special way as it's not structured like a regular Go module.
    "std" = callPackage (
      {
        stdenv,
        hooks,
        go,
      }:
      stdenv.mkDerivation {
        inherit (go) pname version;
        dontUnpack = true;

        nativeBuildInputs = [
          go
          hooks.configureGoCache
          hooks.buildGoCacheOutputSetupHook
        ];

        buildPhase = ''
          runHook preBuild

          # TODO: Move to configure hook
          export GO_NO_VENDOR_CHECKS=1
          export HOME=$(mktemp -d)

          # Perform build of all stdlib packages
          go list ... | xargs -I {} sh -c "go build {} || true"

          runHook postBuild
        '';
      }
    ) { };
  }
)
