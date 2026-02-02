{
  description = "LDStats Development Environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs?ref=nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
      in
      {
        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            # Database
            sqlite

            # Go 
            go
            go-task

            # Version control
            git
            lazygit

            # Pre-commit hooks
            pre-commit

            # Common utilities
            curl
            jq
            ripgrep
            bat
          ];

          shellHook = ''
            echo -e '
                                                                                   
              ███████                   █████                   
            ███░░░░░███                ░░███                    
 ████████  ███     ░░███ ████████    ███████   ██████  ████████ 
░░███░░███░███      ░███░░███░░███  ███░░███  ███░░███░░███░░███
 ░███ ░███░███      ░███ ░███ ░███ ░███ ░███ ░███████  ░███ ░░░ 
 ░███ ░███░░███     ███  ░███ ░███ ░███ ░███ ░███░░░   ░███     
 ░███████  ░░░███████░   ████ █████░░████████░░██████  █████    
 ░███░░░     ░░░░░░░    ░░░░ ░░░░░  ░░░░░░░░  ░░░░░░  ░░░░░     
 ░███                                                           
 █████                                                          
░░░░░                                                           

            '

            # Pre-commit setup
            if [ -f ".pre-commit-config.yaml" ] && command -v pre-commit &> /dev/null; then
                echo "Installing pre-commit hooks..."
                pre-commit install --install-hooks --overwrite || true
            fi
            
            # Show git identity
            GIT_USER=$(git config user.name 2>/dev/null)
            GIT_EMAIL=$(git config user.email 2>/dev/null)
            if [ -n "$GIT_USER" ] && [ -n "$GIT_EMAIL" ]; then
              echo "Git user: $GIT_USER <$GIT_EMAIL>"
              echo ""
            fi

            echo "Development environment ready!"
            echo ""
          '';
        };
      });
}
