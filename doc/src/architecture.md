# Architecture

## Components

- `go/gobuild-nix-gocacheprog`

Responsible for implementing the `GOCACHEPROG` protocol.

- `go/gobuild-nix-generate`

Lock file generator.
Go.sum doesn't contain Nix copmatible hashes & needs to be recomputed.
This component is also responsible for detecting dependency cycles.

- `nix`

Nix code for package set creation & manipulation.
Go programs implementing build time behaviour.

- `default.nix`

The public Nix interface intended to be used by users.

## Design decisions

### Cycle merging

Go modules can have cyclic dependencies, something fundamentally incompatible with Nix which has a DAG build graph.
`gobuild.nix` deals with cyclic dependencies by detecting them at code generation time & only generating a single package for a cycle & building multiple Go modules in the same Nix derivation.

### Indirect vs direct dependencies in the lock file

If all Go packages had run `go mod tidy` as intended we could only dump the direct dependencies of a Go module in the lock, instead of also including the indirect ones.
In practice however many Go packages do not do this and have direct dependencies specified as indirect.

This means we always have to write out all dependencies to the lock.

### Patching of `go.mod` & Symlink farming of `GOMODCACHE`

When creating the module cache directory all `*.mod` files have to be patched & all `go.sum` files have to be omitted from the file tree.

To be more efficient this patching happens at _dependency build time_ and is written to the store as a build output.
When the module cache is "unpacked" it's symlinked in a structure which is as flat as possible.

### Deeply nested indirect dependencies

Because we're building each package in isolation and go.sum doesn't hold the full dependency graph, only the that your application/library cares about.
Without including deeply nested indirect dependencies an application build will fail because the Go compiler needs to be able to look up metadata about it's dependencies, even if those are not required to actually perform the build.

Therefore the lock file generator discovers additional deeply nested dependencies not present in `go.sum` during generation.
