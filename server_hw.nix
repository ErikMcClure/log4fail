{
  server = { ... }: {

    imports = [ <nixpkgs/nixos/modules/profiles/qemu-guest.nix> ];
    networking.hostName = "log4fail-server";

    networking.firewall.allowPing = true;
    services.openssh.enable = true;
    deployment.targetEnv = "digitalOcean";
    deployment.digitalOcean.enableIpv6 = true;
    deployment.digitalOcean.region = "nyc1";
    deployment.digitalOcean.size = "1gb";
  };
  resources.sshKeyPairs.ssh-key = {
    publicKey = builtins.readFile ./serverkey.pub;
    privateKey = builtins.readFile ./serverkey;
  };
}
