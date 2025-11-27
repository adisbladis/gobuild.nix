if [ -z "${NIX_GOBUILD_CACHE_OUT-}" ]; then
  echo "gobuild.nix: Using temporary dummy Go cache output, build cache will be discarded."
  export NIX_GOBUILD_CACHE_OUT=$(mktemp -d)
fi
