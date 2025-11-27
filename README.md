# `gobuild.nix`

Go builders for Nix with per-package fetching & builds.

## Status

Early, not fully baked.

## Motivations

### Incremental fetching & building

`buildGoModule` fetches & builds whole dependency graphs as one big derivation.
This wastes bandwidth, storage space & build time.

By using [`GOCACHEPROG`](https://github.com/golang/go/issues/59719) we can achieve per-package granular builds.
This has the potential to significantly cut down on build times.

## Adopting in nixpkgs

If nixpkgs were to adopt this as it's Go builders, it would imply creating a Go package set.
This has many benefits, but also some challenges.

- Improved security posture

Nixpkgs currently has a weak security posture regarding vulnerable Go dependencies.
Because `buildGoModule` has no insight into the dependency graph it has no actual idea of what libraries are shipped and whether they're vulnerable.

By having only one centrally managed version of a dependency it's easier to ensure we don't ship known vulnerable code.

- Improved composability posture

The "fix" for packages that fail after a compiler version bump is often to pin that package to use an older compiler.
Having a central set which we can patch could make many older compiler pins unnecessary.

- Challenges
  - Tooling would have to be created to keep the set up to date automatically depending on what leaf packages need.
  - It's a dramatic shift away from how `buildGoModule` works
  - Each package needs to record it's build inputs, now they're all grouped into a single hash
  - Much more

## Questions

- Can `buildGoModule` gain caching?

No. `buildGoModule` treats the entire Go dependency graph as a black box, and has no sharing between calls.

## Related

- [Investigate packaging Rust crates separately](https://github.com/NixOS/nixpkgs/issues/333702)
