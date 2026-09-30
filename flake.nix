{
  description = "Development environment for workdeck";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-26.05";

  outputs = { nixpkgs, ... }:
    let
      systems = [ "x86_64-linux" "aarch64-linux" ];
      forAllSystems = f: nixpkgs.lib.genAttrs systems (system: f (import nixpkgs { inherit system; }));
    in
    {
      packages = forAllSystems (pkgs: {
        default = pkgs.buildGoModule {
          pname = "workdeck";
          version = "0.1.0";
          src = ./.;
          subPackages = [ "src" ];
          vendorHash = null;
          postInstall = ''
            mv "$out/bin/src" "$out/bin/workdeck"
          '';
        };
      });

      devShells = forAllSystems (pkgs: {
        default = pkgs.mkShell {
          packages = with pkgs; [ go fzf tmux git ];
        };
      });
    };
}
