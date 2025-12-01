echo "Sourcing configure-go-cache-hook"

goConfigureCache() {
  echo "Executing goConfigureCache"

  if [ -z "${NIX_GOBUILD_CACHE_OUT-}" ]; then
    export NIX_GOBUILD_CACHE_OUT="$out"
  fi

  export GOCACHEPROG=@gocacheprog@

  echo "Finished executing goConfigureCache"
}

if [ -z "${dontUseGoConfigureCache-}" ]; then
  echo "Using goConfigureCache"
  appendToVar preConfigurePhases goConfigureCache
fi
