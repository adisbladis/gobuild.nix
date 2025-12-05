goProxyOutputSetupHook() {
  mkdir -p "$out/nix-support" "$out/goproxy"

  @tool@ buildProxy

  echo "addToSearchPath NIX_GOBUILD_PROXY '$out/goproxy'" >> "$out/nix-support/setup-hook"
}

if [ -z "${dontUseGoProxyOutputSetupHook-}" ]; then
  postPhases+=" goProxyOutputSetupHook"
fi
