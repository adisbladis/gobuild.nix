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

        src = fetchModuleProxy {
          goPackagePath = finalAttrs.pname;
          version = "v${finalAttrs.version}";
          hash = "sha256-nEoB8+Ti+4cNoCVL09shr77C9GSehFgTFJbbU3AFf/U=";
        };

        nativeBuildInputs = [
          hooks.goModuleHook
        ];
      })
    ) { };

    "github.com/alecthomas/assert/v2" = callPackage (
      {
        stdenv,
        hooks,
      }: stdenv.mkDerivation (finalAttrs: {
        pname = "github.com/alecthomas/assert/v2";
        version = "2.11.0";

        src = fetchModuleProxy {
          goPackagePath = finalAttrs.pname;
          version = "v${finalAttrs.version}";
          hash = "sha256-u4hJQUW2Wh4eh9uEpjwEJNEvzo3WB9oJSBgc50BNPqw=";
        };

        nativeBuildInputs = [
          hooks.goModuleHook
        ];

        propagatedBuildInputs = [
          goPackages."github.com/alecthomas/repr"
          goPackages."github.com/hexops/gotextdiff"
        ];
      })
    ) { };

    "github.com/alecthomas/repr" = callPackage (
      {
        stdenv,
        hooks,
      }: stdenv.mkDerivation (finalAttrs: {
        pname = "github.com/alecthomas/repr";
        version = "0.5.2";

        src = fetchModuleProxy {
          goPackagePath = finalAttrs.pname;
          version = "v${finalAttrs.version}";
          hash = "sha256-Wnr0ZxfffuES1HICEC1i0AZ+WOZdOcANeGfEWJkmLr0=";
        };

        nativeBuildInputs = [
          hooks.goModuleHook
        ];
      })
    ) { };

    "github.com/alecthomas/kong" = callPackage (
      {
        stdenv,
        hooks,
        fetchFromGitHub,
        goPackages,
      }:
      stdenv.mkDerivation (finalAttrs: {
        pname = "github.com/alecthomas/kong";
        version = "1.4.0";

        src = fetchModuleProxy {
          goPackagePath = finalAttrs.pname;
          version = "v${finalAttrs.version}";
          hash = "sha256-xOYhjQFvdqsecF//ztyUoRQVyVbSfubbQbMFfKUI2kw=";
        };

        nativeBuildInputs = [
          hooks.goModuleHook
        ];

        propagatedBuildInputs = [
          goPackages."github.com/alecthomas/assert/v2"
        ];
      })
    ) { };

    "github.com/hexops/gotextdiff" = callPackage (
      {
        stdenv,
        hooks,
      }: stdenv.mkDerivation (finalAttrs: {
        pname = "github.com/hexops/gotextdiff";
        version = "1.0.3";

        src = fetchModuleProxy {
          goPackagePath = finalAttrs.pname;
          version = "v${finalAttrs.version}";
          hash = "sha256-PQ5UI9aRbt7n90ptsS3fs00AfGDiS5Kaxm5OJrjwwo0=";
        };

        nativeBuildInputs = [
          hooks.goModuleHook
        ];
      })
    ) { };

    "github.com/fsnotify/fsnotify" = callPackage (
      {
        stdenv,
        hooks,
        fetchFromGitHub,
        goPackages,
      }:
      stdenv.mkDerivation (finalAttrs: {
        pname = "github.com/fsnotify/fsnotify";
        version = "1.8.0";

        src = fetchModuleProxy {
          goPackagePath = "github.com/fsnotify/fsnotify";
          version = "v1.8.0";
          hash = "sha256-xuryvUHfpiQbFPpl2bSJM0Au17RYrZmlAdK6W3KO9Wc=";
        };

        nativeBuildInputs = [
          hooks.goModuleHook
        ];

        propagatedBuildInputs = [
          goPackages."golang.org/x/sys"
        ];
      })
    ) { };
  });

in
{
  fsnotify =
    let
      base = goPackages."github.com/fsnotify/fsnotify";
    in
    pkgs.stdenv.mkDerivation {
      pname = "fsnotify";
      inherit (base) version src;
      buildInputs = [
        base
      ];
      nativeBuildInputs = [
        goPackages.hooks.goAppHook
      ];
    };

  simple-package = pkgs.stdenv.mkDerivation (finalAttrs: {
    name = "simple-package";
    src = ./fixtures/simple-package;
    nativeBuildInputs = [
      goPackages.hooks.goAppHook
    ];
    buildInputs = [
      goPackages."github.com/alecthomas/kong"
    ];
  });
}
