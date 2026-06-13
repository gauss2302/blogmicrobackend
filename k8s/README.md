# Local run on Kubernetes (k3d + Linkerd)

The full stack on a local single-node Kubernetes cluster (k3d) with the Linkerd
service mesh: native CoreDNS discovery, automatic mTLS, an authorization policy
(who-may-call-whom by identity), and retry/timeout traffic policy.

## Prerequisites (one-time)

```bash
brew install k3d linkerd        # kubectl ships with Docker Desktop
```
Docker Desktop running with ~4 GB+ free (the stack peaks around 3.5 GB).

## Up / down

```bash
make k8s-up      # or: bash k8s/up.sh   — build, cluster, Linkerd, deploy (~5–8 min first time)
make k8s-down    # or: bash k8s/down.sh — delete the cluster (manifests stay on disk)
```

Then:
- Frontend: http://localhost:3000
- Gateway:  http://localhost:8080/health
- Mesh dashboard: `linkerd viz dashboard`
- mTLS proof: `linkerd viz edges deployment -n blogmesh` (every edge `SECURED √`)

`up.sh` is idempotent — re-run it after editing manifests or rebuilding images.

## What gets created

| Namespace | Contents |
|---|---|
| `blogmesh` | infra (redis, 3× postgres, rabbitmq, kafka, opensearch) + 7 app services, each meshed (2/2) under its own ServiceAccount |
| `linkerd` / `linkerd-viz` | mesh control plane + dashboard |

## Files

- `secret.yaml` — passwords / connection URLs (dev placeholders; mirrors the gitignored root `.env`)
- `infra.yaml` — datastores + brokers (emptyDir storage, trimmed JVM heaps)
- `apps.yaml` — the 7 services (Deployments + Services + ServiceAccounts), Linkerd-injected
- `policy.yaml` — authorization (lock `user-service` gRPC to its callers) + ServiceProfile (retries + timeout)

## Common tasks

```bash
kubectl get pods -n blogmesh                                   # status (apps are 2/2 = app + linkerd-proxy)
kubectl logs -n blogmesh deploy/api-gateway -c api-gateway     # app logs (-c <name>, the proxy is linkerd-proxy)
kubectl apply -f k8s/                                          # re-apply manifests after edits
kubectl rollout restart deploy -n blogmesh                     # restart everything (sidecars come back automatically)
```

## Other ways to run (without Kubernetes)

- Plain Docker Compose (no mesh): `make up-d` — brings up the whole stack incl. the frontend.
- Consul mesh on Compose (the earlier experiment): `make mesh-up` (+ `make mesh-resync` after recreations). See the repo root.
