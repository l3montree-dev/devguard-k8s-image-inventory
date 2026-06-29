{
  description = "devguard-k8s-image-inventory";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-26.05";
    nixpkgs-unstable.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
    devguard.url = "github:l3montree-dev/devguard";
    devguard.inputs.nixpkgs.follows = "nixpkgs";
    devguard.inputs.nixpkgs-unstable.follows = "nixpkgs-unstable";
    devguard.inputs.flake-utils.follows = "flake-utils";
  };

  outputs = { self, nixpkgs, nixpkgs-unstable, flake-utils, devguard }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        unstablePkgs = nixpkgs-unstable.legacyPackages.${system};
        hostPkgs = nixpkgs.legacyPackages.${system} // {
          buildGoModule = unstablePkgs.buildGoModule;
        };

        targetPkgsAmd64 = nixpkgs.legacyPackages.x86_64-linux // {
          buildGoModule = nixpkgs-unstable.legacyPackages.x86_64-linux.buildGoModule;
        };
        targetPkgsArm64 = nixpkgs.legacyPackages.aarch64-linux // {
          buildGoModule = nixpkgs-unstable.legacyPackages.aarch64-linux.buildGoModule;
        };

        ociAmd64 = import ./nix/oci.nix { pkgs = targetPkgsAmd64; system = "x86_64-linux"; inherit self devguard; };
        ociArm64 = import ./nix/oci.nix { pkgs = targetPkgsArm64; system = "aarch64-linux"; inherit self devguard; };

        binary = import ./nix/k8s-image-inventory.nix {
          buildGoModule = hostPkgs.buildGoModule;
          lib = hostPkgs.lib;
          inherit self system;
        };
      in {
        packages = {
          default = binary;
          devguard-k8s-image-inventory-amd64 = ociAmd64.k8sImageInventoryOCI;
          devguard-k8s-image-inventory-arm64 = ociArm64.k8sImageInventoryOCI;
        };

        devShells.default = hostPkgs.mkShell {
          buildInputs = [
            unstablePkgs.go
            unstablePkgs.gotools
            unstablePkgs.gopls
            unstablePkgs.golangci-lint
          ];
        };
      });
}
