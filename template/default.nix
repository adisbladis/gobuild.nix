{ gobuild-nix, callPackage, stdenv }:
let
  goSet = callPackage gobuild-nix.lib.mkGoSet {
    goLock = ./gobuild-nix.lock;
  };
in
stdenv.mkDerivation {
  name = "gobuild-nix-template";
  src = ./.;
  buildInputs = goSet.require;
  nativeBuildInputs = [
    goSet.hooks.goAppHook
  ];
}
