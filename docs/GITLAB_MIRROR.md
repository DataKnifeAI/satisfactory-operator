# GitLab mirror and Harbor publish

## CI split

| System | Responsibility |
|--------|----------------|
| **GitHub Actions** | Lint, test, compile/build, then mirror `main` to GitLab |
| **GitLab CI** | Build the operator container image and push to Harbor |

Quality gates run on GitHub before mirror. GitLab does **not** re-run lint/unit tests; it only builds and publishes:

`harbor.dataknife.net/library/satisfactory-operator`

Mirror project: [`gitlab.com/dk-raas/dkai/game-servers/satisfactory-operator`](https://gitlab.com/dk-raas/dkai/game-servers/satisfactory-operator).

GitHub source: [`github.com/DataKnifeAI/satisfactory-operator`](https://github.com/DataKnifeAI/satisfactory-operator).

## GitHub workflows

| Workflow | Role |
|----------|------|
| `lint.yml` | `golangci-lint` |
| `test.yml` | `go vet` + `go test ./... -race` |
| `build.yml` | `go build` of `cmd/main.go` |
| `mirror-gitlab.yml` | Re-runs lint + test + build, then pushes to GitLab on `main` |

## GitLab pipeline

`.gitlab-ci.yml` has a single `docker` stage job `publish-satisfactory-operator` that builds with buildkit and pushes `:latest`, `:<commit-sha>`, and `:<git-tag>` when applicable.

Harbor variables are inherited from the `dk-raas/dkai` GitLab group (same as palworld-operator / windrose-operator).

## Deploy smoke test

```bash
# after Harbor publish succeeds
make install
cd config/manager && kustomize edit set image controller=harbor.dataknife.net/library/satisfactory-operator:latest
kubectl apply -k config/default

# sample CR — set ownerId first
kubectl apply -f config/samples/satisfactory_v1alpha1_satisfactoryserver.yaml
kubectl get dws -A
```
