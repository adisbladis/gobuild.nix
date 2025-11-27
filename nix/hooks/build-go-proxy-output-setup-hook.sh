goProxyOutputSetupHook() {
  mkdir -p "$out/nix-support"

  # Link the dependencies we're building into the Go proxy output
  if ! [ -z "$srcs" ]; then
      for srcDir in $srcs; do
          echo "addToSearchPath NIX_GOBUILD_PROXY '$srcDir/cache/download'" >> "$out/nix-support/setup-hook"
      done
  fi
  if ! [ -z "$src" ]; then
      echo "addToSearchPath NIX_GOBUILD_PROXY '$src/cache/download'" >> "$out/nix-support/setup-hook"
  fi
}

if [ -z "${dontUseGoProxyOutputSetupHook-}" ]; then
  postPhases+=" goProxyOutputSetupHook"
fi
