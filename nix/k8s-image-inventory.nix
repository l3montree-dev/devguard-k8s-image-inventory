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
    then "sha256-LpSE4UMAEExUsAJJu4MvESXLATqn4OaGE5utxZuEgK8="
    else "sha256-CZazy2CtT7TmqQ2+5QkvsD/oIJnWd7yx5oG8pW24SsU="; # run `nix build .#devguard-k8s-image-inventory-amd64` to get the linux hash

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
