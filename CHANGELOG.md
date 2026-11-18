# Changelog

## Unreleased

### Added

- Initial `SatisfactoryServer` operator scaffold (Apache-2.0), patterned on
  dragonwilds-operator / palworld-operator.
- Default game image `ghcr.io/robtme/satisfactory-server` with ports 7777 TCP/UDP and
  8888 TCP; PVC at `/config`; `update.skipUpdate` default true.
- LoadBalancer VIP exposure (Envoy Gateway optional / experimental).
- Optional backup CronJob and Server Manager sidecar (inherited patterns).
- Docker Compose path for local smoke without Kubernetes.
