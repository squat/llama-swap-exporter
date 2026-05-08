{
  description = "Promethes exporter for llama-swap and underlying llama.cpp model servers";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    flake-parts.url = "github:hercules-ci/flake-parts";
    git-hooks-nix = {
      url = "github:cachix/git-hooks.nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    systems.url = "github:nix-systems/default";
  };

  outputs =
    {
      self,
      flake-parts,
      git-hooks-nix,
      systems,
      ...
    }@inputs:
    flake-parts.lib.mkFlake { inherit inputs; } {
      imports = [
        git-hooks-nix.flakeModule
      ];
      systems = import systems;
      perSystem =
        {
          pkgs,
          system,
          config,
          ...
        }:
        {
          packages =
            let

              _version = builtins.getEnv "VERSION";
              llama-swap-exporter = pkgs.buildGoModule (finalAttrs: {
                pname = "llama-swap-exporter";
                version = if _version != "" then _version else toString (self.rev or self.dirtyRev or "unknown");
                src = ./.;
                vendorHash = null;
                checkFlags = [ "-skip=^TestE2E" ];
                env.CGO_ENABLED = 0;
                ldflags = [
                  "-s -w -X github.com/squat/llama-swap-exporter/version.Version=${finalAttrs.version}"
                ];

                meta = {
                  description = "Promethes exporter for llama-swap and underlying llama.cpp model servers";
                  mainProgram = "llama-swap-exporter";
                  homepage = "https://github.com/squat/llama-swap-exporter";
                };
              });

            in
            {
              inherit llama-swap-exporter;
              default = llama-swap-exporter;
            }
            // (builtins.listToAttrs (
              map
                (target: {
                  name = "llama-swap-exporter-cross-${target.os}-${target.arch}";
                  value = llama-swap-exporter.overrideAttrs (
                    _: oldAttrs: {
                      env = oldAttrs.env // {
                        GOOS = target.os;
                        GOARCH = target.arch;
                        CGO_ENABLED = 0;
                      };
                      checkPhase = false;
                    }
                  );
                })
                [
                  {
                    os = "linux";
                    arch = "amd64";
                  }
                  {
                    os = "linux";
                    arch = "arm64";
                  }
                  {
                    os = "linux";
                    arch = "arm";
                  }
                ]
            ));

          pre-commit = {
            check.enable = true;
            settings = {
              src = ./.;
              hooks = {
                actionlint.enable = true;
                nixfmt.enable = true;
                nixfmt.excludes = [ "vendor" ];
                gofmt.enable = true;
                gofmt.excludes = [ "vendor" ];
                golangci-lint.enable = true;
                golangci-lint.excludes = [ "vendor" ];
                golangci-lint.extraPackages = [ pkgs.go ];
                govet.enable = true;
                govet.excludes = [ "vendor" ];
                yamlfmt.enable = true;
                yamlfmt.args = [
                  "--formatter"
                  "indentless_arrays=true"
                ];
                yamlfmt.excludes = [
                  ".github"
                  "vendor"
                ];
                readme = {
                  enable = true;
                  name = "README.md";
                  entry =
                    let
                      readmeCheck = pkgs.writeShellApplication {
                        name = "readme-check";
                        text = ''
                          (go run ./... --help 2>&1 1>/dev/null || [ $? -eq 1 ]) | sed 's/\(Usage of\).*\(llama-swap-exporter:\)/\1 \2/' > help.txt
                          go tool embedmd -d README.md
                        '';
                      };
                    in
                    pkgs.lib.getExe readmeCheck;
                  files = "^README\\.md$";
                  extraPackages = [ pkgs.go ];
                };
              };
            };
          };

          devShells = {
            default = pkgs.mkShell {
              inherit (config.pre-commit.devShell) shellHook;
              packages =
                with pkgs;
                [
                  go
                  kind
                  kubectl
                ]
                ++ config.pre-commit.settings.enabledPackages;
            };
          };
        };
    };
}
