{
  description = "uping - A network ping tool";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        packages = {
          uping = pkgs.buildGoModule {
            pname = "uping";
            version = "3.0";
            src = ./src;
            vendorHash = "sha256-IBZvU3qpoHYOq9+yVCkGgjxRv2vBvPRFXBfZJQ9PSec=";
            subPackages = [ "." ];
            doCheck = false;
            # The binary will be named 'uping' by default since main.go is in src
            meta = with pkgs.lib; {
              description = "A network ping tool";
              homepage = "https://github.com/yourusername/uping";
              license = licenses.mit;
              maintainers = with maintainers; [ ];
            };
          };
          default = self.packages.${system}.uping;
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            gopls
            gotools
            go-tools
          ];
        };
      });
} 