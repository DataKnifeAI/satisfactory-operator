# Local (Docker Compose)

```bash
cd compose
cp .env.example .env
make -C .. compose-up   # or: docker compose up -d
```

Connect the Satisfactory client to `127.0.0.1:7777`, claim the server, then create a session.

`./data` mounts to `/config`. Keep this directory to preserve gamefiles and saves across
rebuilds. First start can take several minutes while SteamCMD downloads the server.

```bash
make -C .. compose-down
```
