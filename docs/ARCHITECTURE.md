# Architecture

## Resource graph

```
SatisfactoryServer (CR)
├── Secret          ({name}-secrets)          optional admin-password (SM / future API claim)
├── ConfigMap       ({name}-config)           settings.env (hash-annotated)
├── PVC             ({name}-data)             /config (gamefiles + saves + backups)
├── Deployment      ({name})                  Recreate, robtme image
├── Service         ({name})                  game 7777 TCP+UDP, reliable 8888 TCP
├── PVC             ({name}-backups)          optional tar archives (backup.enabled)
├── CronJob         ({name}-backup)           optional scheduled backup
├── Sidecar         server-manager            optional admin UI (:8088 TCP on VIP)
├── SA/Role/Binding ({name}-server-manager)   RBAC for UI → CR/Secret/restart/backup
├── Service         ({name}-envoy)            ClusterIP backend for routes (Envoy only)
└── Gateway         EnvoyProxy + UDPRoutes    optional — not recommended for Satisfactory
```

Default exposure: `gateway.enabled: false` + `serviceType: LoadBalancer` + `address`
(kube-vip VIP). Envoy UDP proxy rewrites client source IPs and often breaks Unreal joins;
keep Gateway wiring for experiments / parity with other game operators only.

## Design rules

| Rule | Why |
|------|-----|
| No required credentials for start | robtme image starts unclaimed; claim via Satisfactory client HTTPS API |
| Passwords only in Secrets | Never in CR, status, or logs |
| Deployment strategy `Recreate` | Single persistent world — no rolling pairs |
| `terminationGracePeriodSeconds ≥ 30` (default 60) | Clean save-on-stop |
| Config hash annotation | Env/settings changes only roll the pod intentionally |
| PVC retain (document Delete reclaim) | Deleting the CR must not silently destroy worlds |
| Game port not remapped | Satisfactory requires external == container game port |

## Image & updates

Default game image: [`ghcr.io/robtme/satisfactory-server`](https://github.com/robtme/satisfactory-server)
(MIT; wiki-named; formerly wolveix). Coffee Stain does **not** publish an official container.

`spec.update.skipUpdate` defaults to **true** so cluster restarts do not pull multi-GB Steam
updates by surprise. Set `skipUpdate: false` (or omit after flipping the default in a future
release) when you want SteamCMD to update on start. `steamBeta: true` selects experimental.

Optional override: indifferentbroccoli / Harbor rebuild from
[satisfactory-server-k8s](https://github.com/DataKnifeAI/satisfactory-server-k8s) via `spec.image`
(note their default reliable port **7778** vs wiki/robtme **8888**).
