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

    gobuild-nix-gocacheprog = callPackage (
      { stdenv, hooks }:
      stdenv.mkDerivation {
        name = "gobuild-nix-gocacheprog";
        src = ../go/gobuild-nix-gocacheprog;
        nativeBuildInputs = [
          hooks.buildGo
          hooks.installGo
        ];
        meta.mainProgram = "gobuild-nix-gocacheprog";
      }
    ) { };

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

        nativeBuildInputs = [
          hooks.configureGoCache
          hooks.configureGo
          hooks.buildGo
          hooks.buildGoCacheOutputSetupHook
        ];
      }
    ) { };
  }
)
