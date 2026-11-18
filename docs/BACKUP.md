# Backups

Optional CronJob that tars the data PVC into a separate backup PVC.

```yaml
spec:
  backup:
    enabled: true
    schedule: "0 4 * * *"   # UTC; default 04:00 daily
    retain: 7               # keep last N .tar.gz archives
    # size: 20Gi            # default = persistence.size
    # storageClassName: …   # default = persistence.storageClassName
    # image: busybox:1.36
```

## Behaviour

| Resource | Name | Notes |
|----------|------|--------|
| PVC | `{name}-backups` | Survives `backup.enabled: false` so archives are kept |
| CronJob | `{name}-backup` | Deleted when backup is disabled |
| Archive | `/backups/YYYYMMDDTHHMMSSZ.tar.gz` | Full tree under the data mount |

The job mounts the data PVC **read-only** and the backup PVC read-write. Prefer
an RWX/NFS StorageClass (same as the game data volume) so the CronJob can run
while the game pod holds the data claim.

## Restore

1. Stop / scale down the game Deployment (or delete the CR carefully — PVC retain).
2. Copy a chosen archive onto the data PVC (replace contents under the mount).
3. Start the server again.

Example (one-shot restore pod sketch):

```bash
kubectl -n game-servers run dw-restore --rm -it --restart=Never \
  --image=busybox:1.36 \
  --overrides='...'   # mount both PVCs, tar -xzf the archive into /data
```

Treat restores as manual ops — the operator does not auto-rollback worlds.
