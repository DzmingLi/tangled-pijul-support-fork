{
  buildGoApplication,
  modules,
  src,
}:
buildGoApplication {
  pname = "dolly";
  version = "0.1.0";
  inherit src modules;

  # patch the static dir
  postUnpack = ''
    pushd source
    mkdir -p appview/pages/static
    touch appview/pages/static/x
    popd
  '';

  doCheck = false;
  subPackages = ["cmd/dolly"];
}
