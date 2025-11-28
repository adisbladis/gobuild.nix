goCacheOutputSetupHook() {
  mkdir -p "$out/nix-support"
  cat >>"$out/nix-support/setup-hook" <<EOF
addToSearchPath NIX_GOBUILD_CACHE "$out"
EOF
}

if [ -z "${dontUseGoCacheOutputSetupHook-}" ]; then
  postPhases+=" goCacheOutputSetupHook"
fi
