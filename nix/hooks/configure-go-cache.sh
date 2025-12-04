echo "Sourcing configure-go-cache-hook"

goConfigureCache() {
  echo "Executing goConfigureCache"

  export GOCACHEPROG=@gocacheprog@

  if [ -z "${NIX_GOBUILD_CACHE_OUT-}" ]; then
    export NIX_GOBUILD_CACHE_OUT="$out/cache"
  fi

  echo "Finished executing goConfigureCache"
}

if [ -z "${dontUseGoConfigureCache-}" ]; then
  echo "Using goConfigureCache"
  appendToVar preConfigurePhases goConfigureCache
fi
