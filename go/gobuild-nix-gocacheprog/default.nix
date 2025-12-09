{ stdenv, go }:
stdenv.mkDerivation {
  name = "gobuild-nix-gocacheprog";
  src = ./.;
  nativeBuildInputs = [
    go
  ];
  installPhase = ''
    runHook preInstall
    export HOME=$TMPDIR
    env GOMAXPROCS=$NIX_BUILD_CORES GOBIN="$out/bin" go install -v ./...
    runHook postInstall
  '';
  meta.mainProgram = "gobuild-nix-gocacheprog";
}
