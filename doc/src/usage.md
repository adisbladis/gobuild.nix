# Usage

This segment is also available as a Flake template:
```
nix flake init --template github:adisbladis/gobuild.nix
```

## Generating a lock file

To generate a lock file run

```sh
$ gobuild-nix-generate
```

This will download dependencies & process the dependency graph to output `gobuild-nix.lock`.

## Make a derivation

- `default.nix`
```nix
{ gobuild-nix, callPackage, stdenv }:
let
  # Generate a package set from lock & injected dependencies
  # callPackage is responsible for injecting the correct stdenv & other packages
  goSet = callPackage gobuild-nix.lib.mkGoSet {
    goLock = ./gobuild-nix.lock;
  };
in
# Call stdenv.mkDerivation as usual
stdenv.mkDerivation {
  name = "foobar";
  src = ./.;
  # goSet.require contains a list of all top-level dependencies for convenience
  buildInputs = goSet.require;
  nativeBuildInputs = [
    # goSet.goAppHook implements build behaviour for Go applications
    goSet.hooks.goAppHook
  ];
}
```

## Create a development shell

- `shell.nix`
```nix
mkShell {
  packages = [
    go
    gobuild-nix-generate
  ];
}
```
