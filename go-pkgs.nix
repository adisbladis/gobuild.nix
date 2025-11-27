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
        src = ./gobuild-nix-cacher;
        nativeBuildInputs = [
          hooks.buildGo
          hooks.installGo
        ];
        meta.mainProgram = "gobuild-nix-cacher";
      }
    ) { };

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

    # Packages

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
      })
    ) { };

    "github.com/alecthomas/kong" = callPackage (
      {
        stdenv,
        hooks,
        fetchFromGitHub,
        std,
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
          std
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
        ];

        # cp ${finalAttrs.src}/modules.txt vendor/modules.txt

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
  }
)
