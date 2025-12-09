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

    gobuild-nix-gocacheprog = callPackage ../go/gobuild-nix-gocacheprog { };

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
