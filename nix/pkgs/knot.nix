{
  knot-unwrapped,
  makeWrapper,
  git,
  pijul,
}:
knot-unwrapped.overrideAttrs (after: before: {
  nativeBuildInputs = (before.nativeBuildInputs or []) ++ [makeWrapper];

  installPhase = ''
    runHook preInstall

    mkdir -p $out/bin
    cp $GOPATH/bin/knot $out/bin/knot

    wrapProgram $out/bin/knot \
    --prefix PATH : ${git}/bin:${pijul}/bin

    runHook postInstall
  '';
})
