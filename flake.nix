{
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-25.05";
  };
  outputs =
    { ... }@inputs:
    let
      system = "x86_64-linux";
      pkgs = import inputs.nixpkgs { inherit system; };
      drv = pkgs.buildGoModule (finalAttrs: {
        pname = "cors-proxy";
        version = "0.0.1";
        src = ./.;
        vendorHash = null;
      });
    in
    {
      devShells.${system}.default = pkgs.mkShellNoCC {
        packages = with pkgs; [
          go
        ];
        shellHook = ''
          go version
        '';
      };
      packages.${system}.default = drv;
      overlays.default = final: prev: {
        cors-proxy = drv;
      };
    };
}
