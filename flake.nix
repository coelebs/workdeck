{
  description = "Workdeck package and development environment";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-26.05";

  outputs = { nixpkgs, ... }:
    let
      systems = [ "x86_64-linux" "aarch64-linux" ];
      forAllSystems = f: nixpkgs.lib.genAttrs systems (system: f (import nixpkgs { inherit system; }));
    in
    {
      packages = forAllSystems (pkgs: rec {
        default = workdeck;
        workdeck = pkgs.buildGoModule {
          pname = "workdeck";
          version = "0.1.0";
          src = ./.;
          subPackages = [ "src" ];
          vendorHash = null;
          nativeBuildInputs = [ pkgs.makeWrapper ];
          nativeCheckInputs = [ pkgs.fzf pkgs.git ];
          postInstall = ''
            mv "$out/bin/src" "$out/bin/workdeck"
            wrapProgram "$out/bin/workdeck" \
              --prefix PATH : ${pkgs.lib.makeBinPath [ pkgs.fzf pkgs.tmux pkgs.git ]}
          '';
          meta = {
            description = "Tmux work sessions for Git projects and named experiments";
            homepage = "https://github.com/coelebs/workdeck";
            mainProgram = "workdeck";
            platforms = systems;
          };
        };
      });

      devShells = forAllSystems (pkgs: {
        default = pkgs.mkShell {
          packages = with pkgs; [ go fzf tmux git ];
        };
      });
    };
}
