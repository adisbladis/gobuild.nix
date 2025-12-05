echo "Sourcing install-go-hook"

goInstallPhase() {
  echo "Executing goInstallPhase"
  runHook preInstall

  export HOME=$TMPDIR

  @tool@ installGo

  runHook postInstall
  echo "Finished executing goInstallPhase"
}

if [ -z "${dontUseGoInstall-}" ] && [ -z "${installPhase-}" ]; then
  echo "Using goInstallPhase"
  installPhase=goInstallPhase
fi
