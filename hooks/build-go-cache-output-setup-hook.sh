goCacheOutputSetupHook() {
  mkdir -p "$out/nix-support"
  cat >>"$out/nix-support/setup-hook" <<EOF
addToSearchPath NIX_GOCACHE "$out"
EOF
}

if [ -z "${dontUseGoCacheOutputSetupHook-}" ]; then
  postPhases+=" goCacheOutputSetupHook"
fi
