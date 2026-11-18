# Game server

Community image: [`ghcr.io/robtme/satisfactory-server`](https://github.com/robtme/satisfactory-server)  
Wiki: [Dedicated servers](https://satisfactory.wiki.gg/wiki/Dedicated_servers)

## Ports (1.1+)

| Port | Env | Protocol | Usage |
|------|-----|----------|--------|
| 7777 | `SERVERGAMEPORT` | TCP + UDP | Game traffic, HTTPS API, lightweight query |
| 8888 | `SERVERMESSAGINGPORT` | TCP | Reliable messaging |

Port redirection of the **game** port is not supported by the game — external and container
ports must match.

## CR → env map

| Spec | Env |
|------|-----|
| `maxPlayers` | `MAXPLAYERS` |
| `gamePort` | `SERVERGAMEPORT` |
| `reliablePort` | `SERVERMESSAGINGPORT` |
| `update.skipUpdate` (default true) | `SKIPUPDATE` |
| `update.steamBeta` | `STEAMBETA` |
| `puid` / `pgid` | `PUID` / `PGID` |

Server display name and admin password are set when you **claim** the server in the Satisfactory
client (HTTPS API on the game port). They are not robtme startup env vars.

## Persistence

Mount `/config` (PVC). Layout inside the image:

- `gamefiles/` — Steam install (~15Gi+)
- `saved/` — blueprints, saves, server config
- `backups/` — image-managed save backups on start
- `logs/`

Default PVC size: **40Gi**.

## Resources

Wiki recommends ~8–16Gi RAM. Operator defaults request **12Gi** / limit **16Gi** (scales up
with `maxPlayers ≥ 8`).
