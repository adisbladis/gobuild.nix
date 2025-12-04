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

  build-go-proxy-output = callPackage ./build-go-proxy-output { };

in

{
  unpackGoProxy = callPackage (
    { }:
    makeSetupHook {
      name = "unpack-go-proxy-hook";
      substitutions = {
        go = goExe;
        lndir = lib.getExe lndir;
      };
    } ./unpack-go-proxy.sh
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
      };
    } ./build-go.sh
  ) { };

  installGo = callPackage (
    { }:
    makeSetupHook {
      name = "install-go-hook";
      substitutions = {
        go = goExe;
      };
    } ./install-go.sh
  ) { };

  buildGoCacheOutputSetupHook = callPackage (
    { }:
    makeSetupHook {
      name = "build-go-cache-output-setup-hook";
      substitutions = {
        go = goExe;
      };
    } ./build-go-cache-output-setup-hook.sh
  ) { };

  buildGoProxyOutputSetupHook = callPackage (
    { }:
    makeSetupHook {
      name = "build-go-proxy-output-setup-hook";
      substitutions = {
        go = goExe;
        proxybuilder = lib.getExe build-go-proxy-output;
      };
    } ./build-go-proxy-output-setup-hook.sh
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
            hooks.unpackGoProxy
            hooks.configureGo
            hooks.configureGoCache
            hooks.buildGo
            hooks.buildGoCacheOutputSetupHook
            hooks.buildGoProxyOutputSetupHook
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
            hooks.unpackGoProxy
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
