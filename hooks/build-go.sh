echo "Sourcing build-go-hook"

goBuildPhase() {
  echo "Executing goBuildPhase"
  runHook preBuild

  export GO_NO_VENDOR_CHECKS=1
  export HOME=$(mktemp -d)

  @go@ build -v "${goBuildPackages:-./...}"

  runHook postBuild
  echo "Finished executing goBuildPhase"
}

if [ -z "${dontUseGoBuild-}" ] && [ -z "${buildPhase-}" ]; then
  echo "Using goBuildPhase"
  buildPhase=goBuildPhase
fi
