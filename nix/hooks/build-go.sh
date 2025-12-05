echo "Sourcing build-go-hook"

goBuildPhase() {
  echo "Executing goBuildPhase"
  runHook preBuild

  export HOME=$TMPDIR

  @tool@ buildGo

  runHook postBuild
  echo "Finished executing goBuildPhase"
}

if [ -z "${dontUseGoBuild-}" ] && [ -z "${buildPhase-}" ]; then
  echo "Using goBuildPhase"
  buildPhase=goBuildPhase
fi
