# Metrics

The controller manager exposes Prometheus metrics on HTTPS `:8443` (`/metrics`).

## ServiceMonitor

Kubebuilder scaffolds a `ServiceMonitor` under `config/prometheus/`.

Enable it in the default kustomize overlay:

```bash
# in config/default/kustomization.yaml, uncomment:
# - ../prometheus
kubectl apply -k config/default
```

The monitor selects:

```yaml
control-plane: controller-manager
app.kubernetes.io/name: satisfactory-operator
```

Default scrape uses the service-account bearer token and `insecureSkipVerify: true`
(fine for homelab). For production TLS, enable cert-manager and apply
`config/prometheus/monitor_tls_patch.yaml` as documented in that file.

## Manual scrape check

```bash
kubectl -n satisfactory-operator-system port-forward svc/satisfactory-operator-controller-manager-metrics-service 8443:8443
curl -k -H "Authorization: Bearer $(kubectl -n satisfactory-operator-system create token satisfactory-operator-controller-manager)" \
  https://127.0.0.1:8443/metrics | head
```

Controller-runtime / Kubebuilder work-queue metrics (`controller_runtime_*`,
`workqueue_*`) are the primary signal. Game-server health is observed via the
`SatisfactoryServer` status conditions (`Ready`, `GatewayConfigured`, …).
