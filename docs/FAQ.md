# FAQ

## Is there an official Coffee Stain Docker image?

No. Use the community image `ghcr.io/robtme/satisfactory-server` (wiki-recommended) or override
`spec.image` (e.g. indifferentbroccoli / Harbor rebuild).

## Why is skipUpdate default true?

SteamCMD updates on every container start can pull many gigabytes and surprise production
restarts. Flip `spec.update.skipUpdate: false` when you intentionally want an update, then
restart the pod.

## Reliable port 7778 vs 8888?

Wiki and robtme default to **8888**. The indifferentbroccoli / `satisfactory-server-k8s` image
defaults to **7778**. Match the image you run via `spec.reliablePort`.

## Do I need generateSecrets?

Not for a basic server. Claiming happens in the Satisfactory client. Enable
`generateSecrets` (or BYO Secret) when using the optional Server Manager sidecar.

## Pod crash-loops with `chown: Operation not permitted` on `/config`

TrueNAS NFS `root_squash` maps root-created files to `nobody` and blocks `chown`.
The operator runs the robtme container as `PUID`/`PGID` (default 1000) so the
entrypoint skips root chown. Prefer `storageClassName: truenas-csi-nfs-no-mapall`
and do **not** set `fsGroup` on the game pod.
