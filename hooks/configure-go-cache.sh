echo "Sourcing configure-go-cache-hook"

goConfigureCache() {
  echo "Executing goConfigureCache"

  # TODO: Make configurable
  if [ -z "${NIX_GOCACHE_OUT-}" ]; then
    export NIX_GOCACHE_OUT="$out"
  fi

  export NIX_GOCACHE_VERBOSE="1"
  export GOEXPERIMENT="cacheprog"
  export GOCACHEPROG=@cacher@

  echo "Finished executing goConfigureCache"
}

if [ -z "${dontUseGoConfigureCache-}" ]; then
  echo "Using goConfigureCache"
  appendToVar preConfigurePhases goConfigureCache
fi
