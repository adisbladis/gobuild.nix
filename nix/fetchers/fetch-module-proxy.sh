source $stdenv/setup

export HOME=$TMPDIR
export GOMODCACHE=$out
export GOPROXY=${GOPROXY:-https://proxy.golang.org}
export GOSUMDB=off

go mod download "${goPackagePath}@${version}"
