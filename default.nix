{ }:
let
  self = {
    lib = import ./lib.nix;
    packages =
      { callPackage }:
      rec {
        gobuild-nix-generate = callPackage ./go/gobuild-nix-generate {
          gobuild-nix = self;
        };
        generate = gobuild-nix-generate;
      };
  };
in
self
