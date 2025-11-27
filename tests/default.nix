{
  pkgs ?
    let
      flakeLock = builtins.fromJSON (builtins.readFile ../flake.lock);
      inherit (flakeLock.nodes.nixpkgs) locked;
    in
    import (builtins.fetchTree locked) { },
}:

let

  # Go package set containing build cache output & hooks
  goPackages' = pkgs.callPackages ../nix { };

  # Overriden with additional packages
  goPackages = goPackages'.overrideScope (final: prev: let
    inherit (final) callPackage;
    inherit (final.fetchers) fetchModuleProxy;
  in {
    "golang.org/x/sys" = callPackage (
      {
        stdenv,
        hooks,
        fetchgit,
      }:
      stdenv.mkDerivation (finalAttrs: {
        pname = "golang.org/x/sys";
        version = "0.27.0";

        src = fetchgit {
          url = "https://go.googlesource.com/sys";
          rev = "v${finalAttrs.version}";
          hash = "sha256-+d5AljNfSrDuYxk3qCRw4dHkYVELudXJEh6aN8BYPhM=";
        };

        nativeBuildInputs = [
          hooks.configureGoCache
          hooks.buildGo
          hooks.buildGoCacheOutputSetupHook
        ];

        buildInputs = [ final.std ];
      })
    ) { };

    "github.com/alecthomas/kong" = callPackage (
      {
        stdenv,
        hooks,
        fetchFromGitHub,
      }:
      stdenv.mkDerivation {
        pname = "github.com/alecthomas/kong";
        version = "1.4.0";

        src = fetchFromGitHub {
          owner = "alecthomas";
          repo = "kong";
          rev = "v1.4.0";
          hash = "sha256-xfjPNqMa5Qtah4vuSy3n0Zn/G7mtufKlOiTzUemzFcQ=";
        };

        nativeBuildInputs = [
          hooks.configureGoCache
          hooks.buildGo
          hooks.buildGoCacheOutputSetupHook
        ];

        buildInputs = [
          final.std
        ];
      }
    ) { };

    "github.com/fsnotify/fsnotify" = callPackage (
      {
        stdenv,
        hooks,
        fetchFromGitHub,
        goPackages,

      }:
      let
        sys = goPackages."golang.org/x/sys";
      in
      stdenv.mkDerivation {
        pname = "github.com/fsnotify/fsnotify";
        version = "1.8.0";

        src = fetchFromGitHub {
          owner = "fsnotify";
          repo = "fsnotify";
          rev = "v1.8.0";
          hash = "sha256-+Rxg5q17VaqSU1xKPgurq90+Z1vzXwMLIBSe5UsyI/M=";
        };

        nativeBuildInputs = [
          hooks.configureGoCache
          hooks.buildGo
          hooks.buildGoCacheOutputSetupHook
        ];

        buildInputs = [
          sys
          final.std
        ];

        # TODO: Move vendor setup to hook
        preBuild = ''
          mkdir -p vendor/golang.org/x
          ln -s ${sys.src} vendor/golang.org/x/sys

          cat > vendor/modules.txt<<EOF
          # golang.org/x/sys v0.13.0
          ## explicit; go 1.20
          golang.org/x/sys
          EOF

        '';
      }
    ) { };
  });

in
{
  inherit goPackages;

  sys = goPackages."golang.org/x/sys";

  fsnotify =
    let
      base = goPackages."github.com/fsnotify/fsnotify";
    in
    pkgs.stdenv.mkDerivation {
      pname = "fsnotify";
      inherit (base) version src;

      preBuild =
        base.preBuild
        + ''
          export NIX_GOCACHE_OUT=$(mktemp -d)
        '';

      buildInputs = [
        goPackages.std
        goPackages."golang.org/x/sys"
      ];

      nativeBuildInputs =
        let
          inherit (goPackages) hooks;
        in
        [
          hooks.configureGoCache
          hooks.buildGo
          hooks.installGo
        ];
    };

  simple-package = pkgs.stdenv.mkDerivation (finalAttrs: {
    name = "simple-package";

    src = ./fixtures/simple-package;

    nativeBuildInputs =
      let
        inherit (goPackages) hooks;
      in
      [
        hooks.configureGoCache
        hooks.buildGo
        hooks.installGo
      ];

    buildInputs = [
      goPackages."github.com/alecthomas/kong"
      goPackages.std
    ];

    preBuild = ''
      export NIX_GOCACHE_OUT=$(mktemp -d)

      mkdir -p vendor/github.com/alecthomas
      cp modules.txt vendor
      ln -s ${goPackages."github.com/alecthomas/kong".src} vendor/github.com/alecthomas/kong
    '';
  });
}
