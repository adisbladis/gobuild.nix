goModCacheOutputSetupHook() {
  @tool@ buildModCacheOutputSetupHook
}

if [ -z "${dontUseGoModCacheOutputSetupHook-}" ]; then
  postPhases+=" goModCacheOutputSetupHook"
fi
