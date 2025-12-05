echo "Sourcing unpack-go-proxy-hook"

goUnpackPhase() {
  echo "Executing goUnpackPhase"
  runHook preUnpack

  # Go commands needs a working home directory
  if [ -z "${HOME}" ] || [ "${HOME}" = "/homeless-shelter" ] ; then
    export HOME=$TMPDIR
  fi

  # Turn off Go checksum verification
  export GOSUMDB=off

  # Use build-local state as Go proxy
  export GOPROXY="file://${HOME}/go/pkg/mod/cache/download"

  # Unpack Go proxies
  # Creates a `src` directory with either:
  # - User provided inputs
  # - A temporary dummy package
  @tool@ unpackGo

  pushd src > /dev/null

  runHook postUnpack
  echo "Finished executing goUnpackPhase"
}

if [ -z "${dontUseGoUnpack-}" ] && [ -z "${unpackPhase-}" ]; then
  echo "Using goUnpackPhase"
  unpackPhase=goUnpackPhase
fi
