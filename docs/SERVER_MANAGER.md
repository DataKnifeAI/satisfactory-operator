# Server Manager

Optional authenticated admin UI (Palworld-style, Satisfactory-scoped).

```yaml
spec:
  serverManager:
    enabled: true
    port: 8088   # sidecar + optional direct VIP:8088
    hostname: satisfactory.dataknife.net
    httpsPort: 443
    tlsSecretRef:
      name: wildcard-dataknife-net-tls
    # image: harbor.dataknife.net/library/satisfactory-operator:v0.2.1
```

## Access

- HTTPS (Gateway API): `https://<hostname>/` when `hostname` + `tlsSecretRef` are set
- Direct: `http://<VIP>:8088/` still works on the game LoadBalancer Service
- Auth: basic — user `admin`, password = Secret `admin-password`
- Enabling Recreate-rolls the game Deployment (sidecar + Service TCP port)
- HTTPS uses a dedicated Envoy Gateway with `externalIPs=[gateway.address]` so it shares the game VIP without a second kube-vip claim

## Tabs

| Tab | Actions |
|-----|---------|
| Overview | Ready/phase, connect string, world GUID / build |
| Controls | Restart (Recreate), download world data `.tar.gz` |
| Settings | serverName, worldName, players, ownerId, admins → CR |
| Access | Rotate world/admin passwords (Secret); kick/ban stay in-game |
| Updates | `autoUpdateImage`, `validateOnStart` |
| Backups | List/download archives; run backup Job (needs `spec.backup`) |

Settings and password changes require a **restart** to apply into the game process.

## Notes

- No Palworld REST kick/ban — use in-game Server Management as owner.
- Prefer Direct connect for play; the UI is ops, not EOS Recent.
- Sidecar binary is `/server-manager` in the operator image.
