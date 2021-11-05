{ pkgs ? import
  (fetchTarball "http://nixos.org/channels/nixos-unstable/nixexprs.tar.xz") { }
}:

let
  name = "urlshare";
  version = "0.1.0";
  description = "Simple URL sharing";
  homepage = "https://urls.cip.li";
  inherit (pkgs) buildGoModule;
  inherit (pkgs.lib) licenses maintainers platforms;
in buildGoModule {
  pname = name;
  version = version;
  src = builtins.path {
    path = ./.;
    name = name;
  };
  vendorSha256 = null;
  subPackages = [ "." ];

  meta = {
    description = description;
    homepage = homepage;
    license = licenses.mit;
    # maintainers = [ maintainers.sdaros ];
    platforms = platforms.linux;
  };
}
