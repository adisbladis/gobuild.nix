echo "Sourcing build-go-hook"

goBuildPhase() {
  echo "Executing goBuildPhase"
  runHook preBuild

  export HOME=$TMPDIR

  @go@ build -v "${goBuildPackages:-...}"

  runHook postBuild
  echo "Finished executing goBuildPhase"
}

if [ -z "${dontUseGoBuild-}" ] && [ -z "${buildPhase-}" ]; then
  echo "Using goBuildPhase"
  buildPhase=goBuildPhase
fi
