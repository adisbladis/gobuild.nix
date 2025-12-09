# Introduction

Go builders for Nix with per-package fetching & builds.

In nixpkgs `buildGoModule` fetches & builds whole dependency graphs as one big derivation.
This wastes bandwidth, storage space & build time.

By using [`GOCACHEPROG`](https://github.com/golang/go/issues/59719) `gobuild.nix` achieves incremental builds for Go packages within the Nix sandbox.
It works by creating a Nix derivation per Go module dependency, build each in isolation & store the build cache in the Nix store.
