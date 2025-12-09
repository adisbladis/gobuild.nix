# Installation

## Flakes

Add `gobuild-nix` to your inputs:
```nix
{
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs?ref=nixos-unstable";
    gobuild-nix.url = "github:adisbladis/gobuild.nix";
    gobuild-nix.inputs.nixpkgs.follows = "nixpkgs";
  };
}
```

## Classic Nix

You can just as easily import `gobuild.nix` without using Flakes:
``` nix
let
  pkgs = import <nixpkgs> { };

  gobuild-nix = import (builtins.fetchGit {
    url = "https://github.com/adisbladis/gobuild.nix.git";
  }) { };

in ...
```

When using `gobuild.nix` without Flakes you will also have to manually use `callPackage` to access exported packages:
```nix
let
  inherit (callPackage gobuild-nix.packages { }) gobuild-nix-generate;
in ...
```
