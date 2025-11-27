echo "Sourcing unpack-go-proxy-hook"

goUnpackProxyPhase() {
  echo "Executing goUnpackProxyPhase"
  runHook preUnpack

  # Go commands needs a working home directory
  if [ -z "${HOME}" ] || [ "${HOME}" = "/homeless-shelter" ] ; then
    export HOME=$TMPDIR
  fi

  # Turn off Go checksum verification
  export GOSUMDB=off

  # Create Go proxy directory
  mkdir -p .gobuild-proxy
  export GOPROXY=file://$(readlink -f .gobuild-proxy)

  # Link the dependencies we're building into the Go proxy
  if ! [ -z "$srcs" ]; then
      for srcDir in $srcs; do
          if [ -d "$srcDir/cache/download" ]; then
              @lndir@ -silent "$srcDir/cache/download" .gobuild-proxy
          fi
      done
  fi
  if ! [ -z "$src" ]; then
      if [ -d "$srcDir/cache/download" ]; then
          @lndir@ -silent "$src/cache/download" .gobuild-proxy
      fi
  fi

  # Link sources from other builds into proxy
  if ! [ -z "${NIX_GOBUILD_PROXY}" ]; then
    for proxyDir in ${NIX_GOBUILD_PROXY//:/ }; do
      @lndir@ -silent "$proxyDir" .gobuild-proxy
    done
  fi

  # Set up intermediate Go package
  mkdir .gobuild-nix-src
  pushd .gobuild-nix-src > /dev/null
  go mod init gobuild.nix/build > /dev/null

  # Find all Go modules in the Go proxy & `go mod download` them to make
  # them available for our build.
  find ../.gobuild-proxy -name '@v' -type d -prune | while read modProxyDir; do
      local depGoPackagePath=$(echo "$modProxyDir"  | sed 's#\..\/.gobuild-proxy\/##; s#\/@v$##')
      while read depVersion; do
          echo "require $depGoPackagePath $depVersion // indirect" >> go.mod
          go mod download "$depGoPackagePath"
      done < "$modProxyDir/list"
  done

  # If the package we're building is a Go module (as opposed to a module proxy dir)
  # Copy it to the local dir and set ourselves up for the build.
  if [ -e "$src/go.mod" ]; then
      popd >/dev/null
      cp -a "$src" src
      pushd src > /dev/null
  fi

  runHook postUnpack
  echo "Finished executing goUnpackProxyPhase"
}

if [ -z "${dontUseGoUnpackProxy-}" ] && [ -z "${unpackPhase-}" ]; then
  echo "Using goBuildPhase"
  unpackPhase=goUnpackProxyPhase
fi
