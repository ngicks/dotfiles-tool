#!/bin/sh
set -eu
# Rootless docker runs dockerd in a gvisor-tap-vsock network namespace that
# cannot reach systemd-resolved's 127.0.0.53 stub, and dockerd copies
# /etc/resolv.conf at startup. Point the guest resolver at publicly-routable
# DNS (and drop the stub listener) so image pulls can resolve the registry.
mkdir -p /etc/systemd/resolved.conf.d
printf '[Resolve]\nDNS=1.1.1.1 8.8.8.8\nDNSStubListener=no\n' \
  > /etc/systemd/resolved.conf.d/00-podman-static-dns.conf
ln -sf /run/systemd/resolve/resolv.conf /etc/resolv.conf
systemctl restart systemd-resolved
