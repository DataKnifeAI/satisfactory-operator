# Tasks

## Phase 0 — Discovery

- [x] Confirm no official Coffee Stain image; choose robtme as default
- [x] Map ports 7777 TCP/UDP + 8888 TCP and robtme env vars
- [ ] Live cluster smoke: apply CR → claim → join on kube-vip VIP

## Phase 1 — Scaffold & CRD

- [x] Fresh Apache repo from dragonwilds/palworld patterns
- [x] `SatisfactoryServer` API (robtme-oriented)
- [x] Sample CR, Compose parity, docs stubs
- [x] `make generate manifests build test`

## Phase 2 — Core reconciler

- [x] ConfigMap / PVC / Deployment / Service builders
- [x] Optional Secret (Server Manager / future API claim)
- [x] Validation for gateway address + maxPlayers
- [ ] KinD/cluster: apply sample → connect → world on PVC

## Phase 3 — Exposure

- [x] Recommended path: LoadBalancer VIP (`gateway.enabled: false`)
- [x] Experimental Envoy Gateway wiring retained from template
- [ ] Document TCPRoute for reliable port if Envoy path is pursued

## Phase 4 — Ops

- [x] Optional backup CronJob + backup PVC (inherited)
- [x] Optional Server Manager admin UI sidecar (inherited; needs polish)
- [ ] HTTPS API health / auto-claim (oliverziegert inspiration)
- [ ] Image digests / Harbor game-image mirror

## Phase 5 — Launch

- [ ] README polish + Pages site assets
- [ ] CI badges green on GitHub
- [ ] v0.1.0 tag
