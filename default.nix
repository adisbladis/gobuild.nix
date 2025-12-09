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
    ;
  lockSchemaVersion = 1;

  self = {
    mkGoSet =
      {
        goLock,
        go,
        callPackage,
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
                        concatMap (req: if elem req cycle then [ ] else [ final.${req} ]) (
                          locked.require or [ ]
                        )
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

            require = map (packagePath: final.${packagePath}) lockFile.require;
          }
          // mapAttrs (
            goPackagePath: locked:
            final.cycles.${goPackagePath} or (final.callPackage (
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
          ) lockFile.locked;

      in
      (callPackage ./nix { }).overrideScope overlay';
  };

in
self
