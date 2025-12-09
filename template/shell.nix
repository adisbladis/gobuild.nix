{ mkShell, go, gobuild-nix-generate }:
mkShell {
  packages = [
    go
    gobuild-nix-generate
  ];
}
