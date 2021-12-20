{
  server = { pkgs, ... }:
    let log4fail = pkgs.callPackage ./. { };
    in {
      users.users.log4fail = {
        createHome = false;
        group = "log4fail";
        isNormalUser = true;
      };
      users.groups.log4fail = { };
      networking.firewall.allowedTCPPorts = [ 80 443 ];
      networking.firewall.allowedUDPPorts = [ 53 ];

      systemd.services.log4fail = {
        wantedBy = [ "multi-user.target" ];

        serviceConfig = {
          User = "log4fail";
          Group = "log4fail";
          Restart = "on-failure";
        };

        script = ''
          ${log4fail}/bin/log4fail
        '';
      };
    };
}
