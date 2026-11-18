# satisfactory-operator

<p align="center">
  <img src="docs/assets/hero-satisfactory-operator.jpg" alt="Satisfactory Operator — factory world on Kubernetes" width="640">
</p>

<p align="center">
  <a href="https://github.com/DataKnifeAI/satisfactory-operator/actions/workflows/test.yml"><img src="https://github.com/DataKnifeAI/satisfactory-operator/actions/workflows/test.yml/badge.svg" alt="Test"></a>
  <a href="https://github.com/DataKnifeAI/satisfactory-operator/actions/workflows/lint.yml"><img src="https://github.com/DataKnifeAI/satisfactory-operator/actions/workflows/lint.yml/badge.svg" alt="Lint"></a>
  <a href="https://github.com/DataKnifeAI/satisfactory-operator/actions/workflows/build.yml"><img src="https://github.com/DataKnifeAI/satisfactory-operator/actions/workflows/build.yml/badge.svg" alt="Build"></a>
</p>

Kubernetes operator for [Satisfactory](https://www.satisfactorygame.com/) dedicated servers.
One custom resource (`SatisfactoryServer`) reconciles Deployment, PVC, ConfigMap, Service
(and optional Gateway / backup / Server Manager) around the community image
[`ghcr.io/robtme/satisfactory-server`](https://github.com/robtme/satisfactory-server)
(wiki-recommended; MIT; formerly wolveix).

Modeled on [dragonwilds-operator](https://github.com/DataKnifeAI/dragonwilds-operator) and
[palworld-operator](https://github.com/DataKnifeAI/palworld-operator). Complements the older
GPL image fork [satisfactory-server-k8s](https://github.com/DataKnifeAI/satisfactory-server-k8s)
(indifferentbroccoli lineage) — prefer this operator going forward.

**Site:** [dataknifeai.github.io/satisfactory-operator](https://dataknifeai.github.io/satisfactory-operator/)  
**Operator image:** `harbor.dataknife.net/library/satisfactory-operator` (see [docs/GITLAB_MIRROR.md](docs/GITLAB_MIRROR.md))

## Quick start (cluster)

```bash
make manifests generate
make install          # apply CRD
make run              # or deploy the controller image later

kubectl apply -f config/samples/satisfactory_v1alpha1_satisfactoryserver.yaml
# set gateway.address to your kube-vip VIP first
```

Homelab exposure: `gateway.enabled: false`, `serviceType: LoadBalancer`, set `gateway.address`
to a kube-vip VIP, then connect to `status.connectionAddress:7777` (and claim the server in-game).

## Quick start (no cluster)

See [docs/LOCAL.md](docs/LOCAL.md) — Docker Compose with the same robtme image.

## Docs

| Doc | Topic |
|-----|--------|
| [ARCHITECTURE.md](docs/ARCHITECTURE.md) | Resource graph & design rules |
| [GAME_SERVER.md](docs/GAME_SERVER.md) | Ports, env, saves, updates |
| [CONNECT.md](docs/CONNECT.md) | Join / VIP / claim |
| [BACKUP.md](docs/BACKUP.md) | Optional CronJob backups |
| [SERVER_MANAGER.md](docs/SERVER_MANAGER.md) | Optional admin UI sidecar |
| [METRICS.md](docs/METRICS.md) | Prometheus ServiceMonitor |
| [LOCAL.md](docs/LOCAL.md) | Compose path |
| [FAQ.md](docs/FAQ.md) | Common footguns |
| [TASKS.md](docs/TASKS.md) | Phased roadmap |
| [CHANGELOG.md](docs/CHANGELOG.md) | Changes |
| [GITLAB_MIRROR.md](docs/GITLAB_MIRROR.md) | GitHub → GitLab → Harbor |

## Develop

```bash
export PATH="$HOME/sdk/go/bin:$HOME/go/bin:$PATH"
make generate manifests
make test
make build
make lint
```

Requires Go 1.25+ (scaffold targets controller-runtime v0.20).

## License

Apache-2.0 — see [LICENSE](LICENSE).
