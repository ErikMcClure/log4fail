let
  pkgs = import (builtins.fetchGit {
    name = "nixos-stable-2019-09";
    url = "https://github.com/nixos/nixpkgs/";
    ref = "c140d9db0243ea72af77ec43351365a33734b109";
    rev = "";
  }) { };

in pkgs.mkShell { buildInputs = [ pkgs.nixops ]; }
