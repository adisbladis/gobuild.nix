{
  stdenvNoCC,
  cacert,
  git,
  jq,
  lib,
  go,
  curl,
  runCommand,
}:

let
  impureEnvVars = lib.fetchers.proxyImpureEnvVars ++ [ "GOPROXY" ];
in
{
  fetchModuleProxy =
    {
      goPackagePath,
      version,
      hash ? "",
    }:
    stdenvNoCC.mkDerivation {
      name = "${baseNameOf goPackagePath}_${version}";
      builder = ./fetch-module-proxy.sh;
      inherit goPackagePath version impureEnvVars;
      nativeBuildInputs = [
        cacert
        git
        go
        curl
      ];
      outputHashMode = "recursive";
      outputHashAlgo = null;
      outputHash = (
        if hash == "" then
          builtins.warn "found empty hash, assuming '${lib.fakeHash}'" lib.fakeHash
        else
          hash
      );
    };
}
