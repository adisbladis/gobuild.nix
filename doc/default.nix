{
  stdenv,
  mdbook,
  git,
}:

stdenv.mkDerivation {
  pname = "gobuild-nix-docs-html";
  version = "0.1";
  src = ./.;

  nativeBuildInputs = [
    mdbook
    git
  ];

  dontConfigure = true;
  dontFixup = true;

  env.RUST_BACKTRACE = 1;

  buildPhase = ''
    runHook preBuild
    mdbook build
    runHook postBuild
  '';

  installPhase = ''
    runHook preInstall
    mv book $out
    runHook postInstall
  '';
}
