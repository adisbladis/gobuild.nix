let
  wellKnown = builtins.listToAttrs (
    map
      (name: {
        inherit name;
        value = null;
      })
      [
        "go"
        "require"
        "goPackages"
        "gobuild-nix-gocacheprog"
        "fetchers"
        "hooks"
        "callPackage"
      ]
  );
in
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

    # List all non-known attributes in the require list
    require = builtins.concatMap (
      attr:
      if wellKnown ? ${attr} then
        [ ]
      else
        let
          value = final.${attr};
        in
        if !lib.isDerivation value then [ ] else [ value ]
    ) (builtins.attrNames final);

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
