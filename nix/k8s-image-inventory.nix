{ buildGoModule, lib, self }:
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

  vendorHash = lib.fakeHash;

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
