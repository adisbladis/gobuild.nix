if [ -z "${NIX_GOBUILD_CACHE_OUT-}" ]; then
  echo "gobuild.nix: Using $out/cache as Go build cache."
  export NIX_GOBUILD_CACHE_OUT="$out/cache"
fi
