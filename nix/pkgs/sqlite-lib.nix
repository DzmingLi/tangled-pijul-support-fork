{
  stdenv,
  sqlite-lib-src,
}:
stdenv.mkDerivation {
  name = "sqlite-lib";
  src = sqlite-lib-src;

  buildPhase = ''
    $CC -c sqlite3.c
    $AR rcs libsqlite3.a sqlite3.o
    $RANLIB libsqlite3.a
  '';

  installPhase = ''
    mkdir -p $out/include $out/lib
    cp *.h $out/include
    cp libsqlite3.a $out/lib
  '';
}
