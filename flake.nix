{
  description = "A very basic flake";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs?ref=nixos-unstable";
  };

  outputs =
    { self, nixpkgs }:
    let
      inherit (nixpkgs) lib;

      forAllSystems = lib.genAttrs lib.systems.flakeExposed;

      # Import non-flake API
      self' = import self { };
    in
    {
      packages = forAllSystems (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
          (self'.packages {
            inherit (pkgs) callPackage;
          }) // {
            doc = pkgs.callPackage ./doc { };
          }
      );

      inherit (self') lib;

      templates.default = {
        path = ./template;
        description = "A minimal example of using gobuild.nix for development";
      };

      checks = forAllSystems (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        import ./tests { inherit pkgs; }
        // {
          inherit (self.packages.${system}) gobuild-nix-generate;
        }
      );

      devShells = forAllSystems (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        {
          default = pkgs.callPackage ./shell.nix { };
        }
      );

    };
}
