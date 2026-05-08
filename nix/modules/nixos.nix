{
  self,
}:

{
  config,
  lib,
  pkgs,
  utils,
  ...
}:

let
  cfg = config.services.prometheus.exporters.llama-swap;
in
{
  options.services.prometheus.exporters.llama-swap = lib.mkOption {
    type = lib.types.submodule {
      options = {
        enable = lib.mkEnableOption "the Prometheus llama-swap exporter";

        package = lib.mkOption {
          type = lib.types.package;
          default = self.packages.${pkgs.system}.llama-swap-exporter;
          description = "The llama-swap-exporter package to use.";
        };

        port = lib.mkOption {
          type = lib.types.port;
          default = 9293;
          description = "Port to listen on.";
        };

        listenAddress = lib.mkOption {
          type = lib.types.str;
          default = "localhost";
          description = "Address to listen on.";
        };

        upstreams = lib.mkOption {
          type = lib.types.listOf lib.types.str;
          default = [ ];
          example = [ "http://127.0.0.1:8080" ];
          description = ''
            List of llama-swap base URLs to scrape.
            Passed as a comma-separated value to the --upstream flag.
          '';
        };

        extraFlags = lib.mkOption {
          type = lib.types.listOf lib.types.str;
          default = [ ];
          description = "Extra command-line flags to pass to the exporter.";
        };

        environmentFile = lib.mkOption {
          type = lib.types.nullOr lib.types.path;
          default = null;
          example = lib.literalExpression "/run/llama-swap-exporter/env";
          description = ''
            Environment file to load before starting the exporter.
            Use {option}`extraFlags` to pass sensitive CLI arguments
            like `--api-key` directly.
          '';
        };

        openFirewall = lib.mkOption {
          type = lib.types.bool;
          default = false;
          description = "Open port in firewall for incoming connections.";
        };

        firewallFilter = lib.mkOption {
          type = lib.types.nullOr lib.types.str;
          default = null;
          example = "-i eth0 -p tcp -m tcp --dport 9293";
          description = ''
            Filter for iptables when {option}`openFirewall` is true.
            Used as `iptables -I INPUT firewallFilter -j ACCEPT`.
          '';
        };

        firewallRules = lib.mkOption {
          type = lib.types.nullOr lib.types.lines;
          default = null;
          example = ''tcp dport 9293 accept comment "llama-swap-exporter"'';
          description = ''
            nftables rules to add to the input chain when
            {option}`openFirewall` is true.
          '';
        };

        user = lib.mkOption {
          type = lib.types.str;
          default = "llama-swap-exporter";
          description = "User under which the exporter runs.";
        };

        group = lib.mkOption {
          type = lib.types.str;
          default = "llama-swap-exporter";
          description = "Group under which the exporter runs.";
        };
      };
    };
    default = { };
    description = "Configuration for the Prometheus llama-swap exporter.";
  };

  config = lib.mkIf cfg.enable {
    assertions = [
      {
        assertion = cfg.upstreams != [ ];
        message = ''
          services.prometheus.exporters.llama-swap.upstreams must not be empty.
        '';
      }
      {
        assertion = cfg.firewallFilter != null -> cfg.openFirewall;
        message = ''
          services.prometheus.exporters.llama-swap.firewallFilter requires openFirewall to be true.
        '';
      }
      {
        assertion = cfg.firewallRules != null -> cfg.openFirewall;
        message = ''
          services.prometheus.exporters.llama-swap.firewallRules requires openFirewall to be true.
        '';
      }
    ];

    systemd.services."prometheus-llama-swap-exporter" = {
      description = "Prometheus llama-swap Exporter";
      after = [ "network.target" ];
      wantedBy = [ "multi-user.target" ];

      serviceConfig =
        let
          enableDynamicUser = cfg.user == "llama-swap-exporter";
        in
        {
          Type = "simple";
          Restart = "always";
          PrivateTmp = true;
          WorkingDirectory = "/tmp";
          DynamicUser = lib.mkIf enableDynamicUser true;
          User = lib.mkIf (!enableDynamicUser) cfg.user;
          Group = lib.mkIf (!enableDynamicUser) cfg.group;
          EnvironmentFile = lib.mkIf (cfg.environmentFile != null) [ cfg.environmentFile ];

          ExecStart = lib.escapeShellArgs (
            [
              (lib.getExe cfg.package)
              "--web.listen-address"
              "${cfg.listenAddress}:${toString cfg.port}"
              "--upstream"
              (lib.concatStringsSep "," cfg.upstreams)
            ]
            ++ cfg.extraFlags
          );

          # Hardening
          CapabilityBoundingSet = [ "" ];
          DeviceAllow = [ "" ];
          LockPersonality = true;
          MemoryDenyWriteExecute = true;
          NoNewPrivileges = true;
          PrivateDevices = true;
          ProtectClock = true;
          ProtectControlGroups = true;
          ProtectHome = true;
          ProtectHostname = true;
          ProtectKernelLogs = true;
          ProtectKernelModules = true;
          ProtectKernelTunables = true;
          ProtectSystem = "strict";
          RemoveIPC = true;
          RestrictAddressFamilies = [
            "AF_INET"
            "AF_INET6"
          ];
          RestrictNamespaces = true;
          RestrictRealtime = true;
          RestrictSUIDSGID = true;
          SystemCallArchitectures = "native";
          UMask = "0077";
        };
    };

    networking.firewall = lib.mkIf cfg.openFirewall {
      extraCommands = lib.mkIf (!config.networking.nftables.enable) (
        lib.concatStringsSep " " [
          "iptables -A INPUT"
          (cfg.firewallFilter or "-p tcp -m tcp --dport ${toString cfg.port}")
          "-m comment --comment llama-swap-exporter"
          "-j ACCEPT"
        ]
      );
      extraInputRules = lib.mkIf (config.networking.nftables.enable) (
        cfg.firewallRules or "tcp dport ${toString cfg.port} accept comment \"llama-swap-exporter\""
      );
    };
  };
}
