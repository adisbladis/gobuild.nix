{
  pkgs ?
    let
      flakeLock = builtins.fromJSON (builtins.readFile ./flake.lock);
      inherit (flakeLock.nodes.nixpkgs) locked;
    in
    import (builtins.fetchTree locked) { },
}:
# Note: Commented out lines below are for dogfooding of using Nix provided
# caches in development.
# I'm commenting this out for now to have more freedom to break things,
# but once we're more stabilised I'm putting it back.

pkgs.mkShell {
  packages = [
    pkgs.go
    pkgs.mdbook
    # cacher
  ];

  # env = {
  #   GOEXPERIMENT = "cacheprog";
  #   # GOCACHEPROG = pkgs.lib.getExe cacher;
  # };
}
