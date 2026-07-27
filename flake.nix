{
  description = "Alertmanager webhook to MQTT bridge";

  inputs = {
    naersk.url = "github:nix-community/naersk/master";
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, utils, naersk }:
    utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
        name = "alertmanager-mqtt-bridge";
        imageSource =
          let fromEnv = builtins.getEnv "IMAGE_SOURCE";
          in if fromEnv != "" then fromEnv else "https://github.com/roberteggl/Alertmanager-Webhook-MQTT-Bridge";
        imageRevision =
          let fromEnv = builtins.getEnv "IMAGE_REVISION";
          in if fromEnv != "" then fromEnv else (self.rev or self.dirtyRev or "unknown");

        goBuild = pkgs.buildGoModule {
          pname = name;
          version = "0.1.0";
          src = ./.;
          subPackages = [ "." ];
          vendorHash = "sha256-CYNAQ2CoMCB9ha1DSSgjRv4vJqqBOHImm8k8tvwAsTc=";
        };

        # The actual binary name (Go uses directory/module name)
        binaryName = "Alertmanager-Webhook-MQTT-Bridge";

        # Create a package that has the binary in /bin
        appPackage = pkgs.runCommand "${name}-app" {} ''
          mkdir -p $out/bin
          cp ${goBuild}/bin/${binaryName} $out/bin/${binaryName}
          chmod +x $out/bin/${binaryName}
        '';

        dockerImage = pkgs.dockerTools.buildImage {
          name = name;
          tag = "latest";
          copyToRoot = pkgs.buildEnv {
            name = "image-root";
            paths = [
              pkgs.cacert
              appPackage
              pkgs.bash
            ];
            pathsToLink = [ "/bin" "/etc" ];
          };
          config = {
            Entrypoint = [ "/bin/${binaryName}" ];
            ExposedPorts = { "8080/tcp" = { }; };
            WorkingDir = "/";
            Env = [
              "SSL_CERT_FILE=${pkgs.cacert}/etc/ssl/certs/ca-bundle.crt"
            ];
            Labels = {
              "org.opencontainers.image.source" = imageSource;
              "org.opencontainers.image.revision" = imageRevision;
            };
          };
        };
      in
      {
        defaultPackage = goBuild;
        packages = {
          default = goBuild;
          alertmanager-mqtt-bridge = goBuild;
          dockerImage = dockerImage;
        };

        defaultApp = {
          type = "app";
          program = "${goBuild}/bin/${name}";
        };
      });
}
