{ pkgs, self, devguard }:
let
  common = import ./common.nix { inherit self; };

  binary = import ./k8s-image-inventory.nix {
    buildGoModule = pkgs.buildGoModule;
    lib = pkgs.lib;
    inherit self;
  };

  trivyFromSource = pkgs.callPackage "${devguard}/nix/trivy.nix" { };
in
{
  k8sImageInventoryOCI = pkgs.dockerTools.buildLayeredImage {
    name = "devguard-k8s-image-inventory";
    tag = common.version;

    contents = [
      pkgs.cacert
      binary
      trivyFromSource
    ];

    fakeRootCommands = ''
      mkdir -p tmp
      chmod 1777 tmp
    '';
    enableFakechroot = true;

    config = {
      Cmd = [ "/bin/devguard-k8s-image-inventory" ];
      User = "53111:53111";
      Env = [
        "SSL_CERT_FILE=/etc/ssl/certs/ca-bundle.crt"
        "TRIVY_CACHE_DIR=/tmp/.cache/trivy"
        "TRIVY_CONFIG=/etc/devguard/trivy.yaml"
      ];
    };
  };
}
