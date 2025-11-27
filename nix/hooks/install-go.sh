echo "Sourcing install-go-hook"

goInstallPhase() {
  echo "Executing goInstallPhase"
  runHook preInstall

  if [ -z "${goInstallPackages}" ]; then
      export goInstallPackages="./..."
  fi

  env GOBIN="$out/bin" @go@ install -v $goInstallFlags $goInstallPackages
  if ! [ -e "$out" ] || [ $(ls "$out" | wc -l) -eq 0 ]; then
      echo "build failure: goInstallPhase failed to produce any outputs in $out" >> /dev/stderr
      echo "hint: set goInstallPackages" >> /dev/stderr
      exit 1
  fi

  runHook postInstall
  echo "Finished executing goInstallPhase"
}

if [ -z "${dontUseGoInstall-}" ] && [ -z "${installPhase-}" ]; then
  echo "Using goInstallPhase"
  installPhase=goInstallPhase
fi
