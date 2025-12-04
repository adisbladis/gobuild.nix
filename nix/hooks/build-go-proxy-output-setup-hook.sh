goProxyOutputSetupHook() {
  mkdir -p "$out/nix-support" "$out/goproxy"

  @proxybuilder@

  echo "addToSearchPath NIX_GOBUILD_PROXY '$out/goproxy/cache/download'" >> "$out/nix-support/setup-hook"
}

if [ -z "${dontUseGoProxyOutputSetupHook-}" ]; then
  postPhases+=" goProxyOutputSetupHook"
fi
