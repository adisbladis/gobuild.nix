let
  inherit (builtins)
    isAttrs
    fromTOML
    readFile
    concatMap
    elem
    groupBy
    attrNames
    mapAttrs
    genericClosure
    ;
  lockSchemaVersion = 1;

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

          # Cycle packages by their group IDs
          cyclesByGroup =
            mapAttrs
              (
                cycleIdx: cycle:
                final.callPackage (
                  {
                    stdenv,
                    fetchers,
                    hooks,
                  }:
                  stdenv.mkDerivation {
                    name = "go-cycle-${cycleIdx}";

                    srcs = map (
                      goPackagePath:
                      let
                        locked = lockFile.locked.${goPackagePath};
                      in
                      fetchers.fetchModuleProxy {
                        inherit goPackagePath;
                        inherit (locked) version hash;
                      }
                    ) cycle;

                    nativeBuildInputs = [
                      hooks.goModuleHook
                    ];

                    propagatedBuildInputs = concatMap (
                      goPackagePath:
                      let
                        locked = lockFile.locked.${goPackagePath};
                      in
                      concatMap (req: if elem req cycle then [ ] else [ final.${req} ]) (locked.require or [ ])
                    ) cycle;

                  }
                ) { }
              )
              (
                # Attrset of cycles by their numeric group -> list of members
                groupBy (n: toString cycles.${n}) (attrNames cycles)
              );

          # Map goPackagePath -> cycle package
          cyclePkgs = mapAttrs (goPackagePath: cycleGroup: cyclesByGroup.${toString cycleGroup}) final.cycles;

        in
        {
          inherit cycles;

          require = map (goPackagePath: final.${goPackagePath}) (attrNames lockFile.locked);
        }
        //
          # Create a package per Go _module_
          mapAttrs (
            goPackagePath: locked:
            cyclePkgs.${goPackagePath} or (final.callPackage (
              {
                stdenv,
                fetchers,
                hooks,
              }:
              stdenv.mkDerivation {
                name = goPackagePath;
                inherit (locked) version;

                src = fetchers.fetchModuleProxy {
                  inherit goPackagePath;
                  inherit (locked) version hash;
                };

                passthru = {
                  inherit cycles;
                  inherit cyclePkgs;
                };

                nativeBuildInputs = [
                  hooks.goModuleHook
                ];

                propagatedBuildInputs = map (depGoPackagePath: final.${depGoPackagePath} or null) (
                  locked.require or [ ]
                );

              }
            ) { })
          ) lockFile.locked
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
                  src = lib.sources.sourceByGlobs rootDir ([
                    "go.mod"
                    "go.work.sum"
                  ] ++ map (dir: "${dir}/**") dirs);

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
