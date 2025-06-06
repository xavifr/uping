{
  description = "uping - A network ping tool";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs = { self, nixpkgs }:
    let
      supportedSystems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];
      forAllSystems = nixpkgs.lib.genAttrs supportedSystems;
    in
    {
      packages = forAllSystems (system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        {
          uping = pkgs.buildGoModule {
            pname = "uping";
            version = "3.0";
            src = ./src;
            vendorHash = "sha256-IBZvU3qpoHYOq9+yVCkGgjxRv2vBvPRFXBfZJQ9PSec=";
            subPackages = [ "." ];
            doCheck = false;
            meta = with pkgs.lib; {
              description = "A network ping tool";
              homepage = "https://github.com/xavifr/uping";
              license = licenses.mit;
              maintainers = with maintainers; [ ];
              platforms = platforms.unix;
            };
          };
          default = self.packages.${system}.uping;
        });

      devShells = forAllSystems (system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        {
          default = pkgs.mkShell {
            buildInputs = with pkgs; [
              go
              gopls
              gotools
              go-tools
            ];
          };
        });

      # Add nixosModule for system-wide installation
      nixosModules.default = { config, lib, pkgs, ... }:
        with lib;
        let
          cfg = config.services.uping;
        in
        {
          options.services.uping = {
            enable = mkEnableOption "uping service";
          };

          config = mkIf cfg.enable {
            environment.systemPackages = [ self.packages.${pkgs.system}.uping ];
          };
        };
    };
} 