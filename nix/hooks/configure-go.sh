echo "Sourcing configure-go-hook"

goConfigurePhase() {
  echo "Executing goConfigurePhase"
  runHook preConfigure

  # No configure phase behaviour provided at the moment.
  # This hook mainly exists to opt out of the default nixpkgs
  # configurePhase behaviour.

  runHook postConfigure
  echo "Finished executing goConfigurePhase"
}

if [ -z "${dontUseGoConfigure-}" ] && [ -z "${configurePhase-}" ]; then
  echo "Using goConfigurePhase"
  configurePhase=goConfigurePhase
fi
