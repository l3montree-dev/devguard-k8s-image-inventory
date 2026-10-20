{ buildGoModule, lib, self, system }:
let
  common = import ./common.nix { inherit self; };
in
buildGoModule {
  pname = "devguard-k8s-image-inventory";
  version = common.version;

  src = lib.fileset.toSource {
    root = ../.;
    fileset = lib.fileset.unions [
      ../go.mod
      ../go.sum
      (lib.fileset.fileFilter (f: f.hasExt "go") ../.)
    ];
  };

  # vendorHash differs per OS because `go mod vendor` applies build constraints.
  vendorHash = if lib.hasSuffix "-darwin" system
    then "sha256-xCtmENEcf4EozlHfjlOpJz8cpWpRgiMEZZbPmwwU5Lo="
    else "sha256-e+n1xdEAv6JGLAi2DsT71nbPXjBpbrOZsntT9yZMSaQ="; # run `nix build .#devguard-k8s-image-inventory-amd64` to get the linux hash

  ldflags = [
    "-s"
    "-w"
    "-X main.Version=${common.version}"
    "-X main.Commit=${common.commit}"
    "-X main.Date=${common.buildDate}"
    "-X main.BuiltBy=nix"
  ];

  doCheck = false;
  env.CGO_ENABLED = 0;
}
