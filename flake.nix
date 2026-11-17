{
  description = "uping - A network ping tool";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs = { self, nixpkgs }:
    let
      supportedSystems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];
      forAllSystems = nixpkgs.lib.genAttrs supportedSystems;

      # src/go.mod declares the minimum Go version, for example "go 1.26.0".
      # Reading it means the flake follows the module instead of silently
      # building with whatever Go the locked nixpkgs happens to provide, which
      # is what broke the build when the dependencies were updated.
      goModLines = builtins.filter builtins.isString (
        builtins.split "\n" (builtins.readFile ./src/go.mod)
      );

      goVersionMatches = builtins.filter (m: m != null) (
        map (line: builtins.match "go ([0-9]+\\.[0-9]+).*" line) goModLines
      );

      goVersion =
        if goVersionMatches == [ ]
        then throw "could not read the go directive from src/go.mod"
        else builtins.head (builtins.head goVersionMatches);

      # Prefer the matching nixpkgs attribute (go_1_26), then fall back to the
      # default Go if it is new enough, and otherwise fail with an actionable
      # message rather than a confusing vendor hash error.
      goFor = pkgs:
        if pkgs ? "go_${nixpkgs.lib.replaceStrings [ "." ] [ "_" ] goVersion}"
        then pkgs."go_${nixpkgs.lib.replaceStrings [ "." ] [ "_" ] goVersion}"
        else if nixpkgs.lib.versionAtLeast pkgs.go.version goVersion
        then pkgs.go
        else throw ''
          nixpkgs provides Go ${pkgs.go.version}, but src/go.mod requires ${goVersion}.
          Update the nixpkgs input (nix flake update) or pin a newer Go.
        '';

      # The unversioned buildGoModule is an alias for one specific
      # buildGoNNModule, whose `go` argument is already pinned (see
      # pkgs/top-level/all-packages.nix). Passing `go = ...` at call time is
      # therefore silently ignored; the toolchain has to be replaced through
      # `.override` on the builder itself.
      buildGo = pkgs: (pkgs.buildGoModule.override { go = goFor pkgs; });
    in
    {
      packages = forAllSystems (system:
        let
          pkgs = nixpkgs.legacyPackages.${system};
        in
        {
          uping = (buildGo pkgs) {
            pname = "uping";
            version = "3.1.0";
            src = ./src;

            # Hash of the vendor tree derived from go.mod/go.sum. This cannot be
            # avoided: src/vendor is gitignored, so Nix materialises the vendor
            # tree itself and needs to pin what it produced.
            #
            # Refresh after ANY dependency change with:
            #   nix build .#uping --no-link 2>&1 | grep 'got:'
            # and paste the reported value here. (Setting vendorHash to
            # pkgs.lib.fakeHash forces the correct value to be printed.)
            vendorHash = "sha256-z/VsrhjqiebMxiXHEFqs9/PkknZyXCSvRl0XWeMmZqg=";

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
              (goFor pkgs)
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
