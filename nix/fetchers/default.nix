{
  stdenvNoCC,
  cacert,
  git,
  jq,
  lib,
  go,
  curl,
  runCommand,
  writeScript,
}:

let
  impureEnvVars = lib.fetchers.proxyImpureEnvVars ++ [ "GOPROXY" ];

  # This is embedded into the expression so the expression can be embedded into the generator program
  fetchModuleProxy = writeScript "fetch-module-proxy.sh" ''
    source $stdenv/setup

    export HOME=$TMPDIR
    export GOMODCACHE=$out
    export GOPROXY=''${GOPROXY:-https://proxy.golang.org}
    export GOSUMDB=off

    go mod download "''${goPackagePath}@''${version}"
  '';

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
      builder = fetchModuleProxy;
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
