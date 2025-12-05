{
  callPackage,
  makeSetupHook,
  go,
  gobuild-nix-gocacheprog,
  lib,
  lndir,
}:
let
  goExe = lib.getExe go;

  gobuild-nix-tool = callPackage ./gobuild-nix-tool { };
  tool = lib.getExe gobuild-nix-tool;

in

{
  inherit gobuild-nix-tool;

  unpackGo = callPackage (
    { }:
    makeSetupHook {
      name = "unpack-go-hook";
      substitutions = {
        inherit tool;
      };
    } ./unpack-go.sh
  ) { };

  configureGoCache = callPackage (
    { }:
    makeSetupHook {
      name = "configure-go-cache-hook";
      substitutions = {
        gocacheprog = lib.getExe gobuild-nix-gocacheprog;
        go = goExe;
      };
    } ./configure-go-cache.sh
  ) { };

  configureGo = callPackage (
    { }:
    makeSetupHook {
      name = "configure-go-hook";
      substitutions = {
        go = goExe;
      };
    } ./configure-go.sh
  ) { };

  buildGo = callPackage (
    { }:
    makeSetupHook {
      name = "build-go-hook";
      substitutions = {
        go = goExe;
        inherit tool;
      };
    } ./build-go.sh
  ) { };

  installGo = callPackage (
    { }:
    makeSetupHook {
      name = "install-go-hook";
      substitutions = {
        go = goExe;
        inherit tool;
      };
    } ./install-go.sh
  ) { };

  buildGoCacheOutputSetupHook = callPackage (
    { }:
    makeSetupHook {
      name = "build-go-cache-output-setup-hook";
    } ./build-go-cache-output-setup-hook.sh
  ) { };

  buildGoModCacheOutputSetupHook = callPackage (
    { }:
    makeSetupHook {
      name = "build-go-modcache-output-setup-hook";
      substitutions = {
        inherit tool;
      };
    } ./build-go-modcache-setup-hook.sh
  ) { };

  goModuleHook = callPackage
    (
      { hooks, std }:
        makeSetupHook {
          name = "go-module-hook";
          passthru = {
            inherit go;
          };
          propagatedBuildInputs = [
            go
            std
            hooks.unpackGo
            hooks.configureGo
            hooks.configureGoCache
            hooks.buildGo
            hooks.buildGoCacheOutputSetupHook
            hooks.buildGoModCacheOutputSetupHook
          ];
        } ./module-hook.sh
    )
    {
    };

  goAppHook = callPackage
    (
      { hooks, std }:
        makeSetupHook {
          name = "go-app-hook";
          passthru = {
            inherit go;
          };
          propagatedBuildInputs = [
            go
            std
            hooks.unpackGo
            hooks.configureGo
            hooks.configureGoCache
            # Note that explicitly calling `go build` is not required.
            # Building is implied by installing, so we don't need to do both.
            # hooks.buildGo
            hooks.installGo
          ];
        } ./app-hook.sh
    )
    {
    };
}
