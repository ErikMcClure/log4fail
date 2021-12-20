{ pkgs ? import <nixpkgs> { }, ... }:

pkgs.buildGoModule {
  name = "log4fail";
  src = ./.;
  vendorSha256 = "1q68i187m5kga191pvr5wfa80g2hr869cqr1j4xrcqkaz5v0pxrk";
}
