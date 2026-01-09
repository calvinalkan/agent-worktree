{
  description = "wt - git worktree manager for agentic coding workflows";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-25.11";
  };

  outputs = { self, nixpkgs }:
    let
      system = "x86_64-linux";
      pkgs = nixpkgs.legacyPackages.${system};
    in
    {
      packages.${system}.default = pkgs.buildGoModule {
        pname = "wt";
        version = "0.1.0";
        src = ./.;
        vendorHash = "sha256-9HfyAMKzZVAh+1wRDZcEPa6oma8oOIp8hn47Wc7FZzk=";
        doCheck = false;  # Tests need git, skip for now
      };
    };
}
