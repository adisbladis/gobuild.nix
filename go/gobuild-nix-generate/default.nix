{ gobuild-nix, callPackage, stdenv }:
let
  goSet = callPackage gobuild-nix.mkGoSet {
    goLock = ./gobuild-nix.lock;
  };
in
stdenv.mkDerivation {
  name = "gobuild-nix-generate";
  src = ./.;
  postUnpack = ''
    cp ${../../nix/fetchers/default.nix} fetcher.nix
  '';
  buildInputs = [
    goSet."github.com/BurntSushi/toml"
    goSet."golang.org/x/mod"
    goSet."golang.org/x/sync"
  ];
  nativeBuildInputs = [
    goSet.hooks.goAppHook
  ];
}
