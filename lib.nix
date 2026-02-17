let
  inherit (builtins)
    isAttrs
    fromTOML
    readFile
    concatMap
    attrNames
    mapAttrs
    genericClosure
    pathExists
    ;
  lockSchemaVersion = 1;

  optionalFile = filepath: if pathExists filepath then [ filepath ] else [ ];

in
{
  mkGoSet =
    {
      goLock,
      go,
      callPackage,
      lib,
      overridePackage ? drv: drv,
      rootDir ? throw "Local package was used but no root directory passed",
    }:
    let
      lockFile = if isAttrs goLock then goLock else fromTOML (readFile goLock);

      overlay' =
        assert lockFile.schema == lockSchemaVersion;
        final: prev:
        let
          cycles = lockFile.cycles or { };

          # Single derivation that bundles all external modules together.
          allModules = final.callPackage (
            {
              stdenv,
              fetchers,
              hooks,
            }:
            stdenv.mkDerivation {
              name = "go-all-modules";

              srcs = map (
                goPackagePath:
                let
                  locked = lockFile.locked.${goPackagePath};
                in
                fetchers.fetchModuleProxy {
                  inherit goPackagePath;
                  inherit (locked) version hash;
                }
              ) (attrNames lockFile.locked);

              nativeBuildInputs = [
                hooks.goModuleHook
              ];

              dontUseGoBuild = "1";
              dontUseGoCacheOutputSetupHook = "1";
            }
          ) { };

        in
        {
          inherit cycles;

          require = [ final.allModules ];

          allModules = allModules;
        }
        //
          mapAttrs (_goPackagePath: _locked: allModules) lockFile.locked
        //
          # Create a package per local Go _package_
          mapAttrs (
            goPackagePath: locked:
            let
              # Resolve local package requirements
              require = genericClosure {
                startSet = [ { key = goPackagePath; } ];
                operator =
                  item:
                  concatMap (
                    goPackagePath: if !lockFile.package ? ${goPackagePath} then [ ] else [ { key = goPackagePath; } ]
                  ) (lockFile.package.${item.key}.require or [ ]);
              };

              # Local package directories to include
              dirs = map (item: lockFile.package.${item.key}.dir) require;

            in
            overridePackage (
              final.callPackage (
                {
                  stdenv,
                  hooks,
                }:
                stdenv.mkDerivation {
                  name = goPackagePath;

                  # Create a union of all required local sources
                  src = lib.fileset.toSource {
                    root = rootDir;
                    fileset = (
                      lib.fileset.unions (
                        (optionalFile (rootDir + "/go.mod"))
                        ++ (optionalFile (rootDir + "/go.work"))
                        ++ map (dir: rootDir + dir) dirs
                      )
                    );
                  };

                  # Only build the current Go package
                  env.goBuildPackages = goPackagePath + "/...";

                  nativeBuildInputs = [
                    hooks.goPackageHook
                  ];

                  passthru = {
                    inherit goPackagePath;
                  };

                  propagatedBuildInputs =
                    final.require ++ map (depGoPackagePath: final.${depGoPackagePath} or null) (locked.require or [ ]);
                }
              ) { }
            )
          ) (lockFile.package or { });

    in
    (callPackage ./nix { inherit go; }).overrideScope overlay';
}
