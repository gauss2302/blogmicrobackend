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

## Layout (Kustomize)

```
k8s/
  base/                  # env-neutral: namespace, infra, apps, mesh policy
  overlays/local/        # k3d: + dev Secret (NodePort/emptyDir/:dev images come from base)
  overlays/production/   # VPS: Ingress+TLS, PVCs, ghcr.io images, replicas, SealedSecret
  linkerd/               # cert-manager-managed Linkerd identity (production)
```

- `base/infra.yaml` — datastores + brokers (emptyDir storage, trimmed JVM heaps)
- `base/apps.yaml` — the 7 services (Deployments + Services + ServiceAccounts), Linkerd-injected
- `base/policy.yaml` — authorization (each backend's gRPC/HTTP locked to the gateway identity) + ServiceProfile
- `overlays/local/secret.yaml` — dev placeholder passwords/URLs (mirrors the gitignored root `.env`)

`make k8s-up` applies `overlays/local`. Preview either overlay offline with
`make render-local` / `make render-prod`.

## Common tasks

```bash
kubectl get pods -n blogmesh                                   # status (apps are 2/2 = app + linkerd-proxy)
kubectl logs -n blogmesh deploy/api-gateway -c api-gateway     # app logs (-c <name>, the proxy is linkerd-proxy)
kubectl apply -k k8s/overlays/local                           # re-apply manifests after edits
kubectl rollout restart deploy -n blogmesh                     # restart everything (sidecars come back automatically)
```

## Production

A real single-VPS **k3s** deployment — Ingress + Let's Encrypt TLS, PVC
persistence + nightly DB backups, auto-rotating Linkerd certs (cert-manager),
ghcr.io images via CI — is **[`docs/production.md`](../docs/production.md)**.
After a short one-time setup it's one command: `make prod-bootstrap`, then
`make prod-deploy` for subsequent releases.

## Other ways to run (without Kubernetes)

- Plain Docker Compose (no mesh): `make up-d` — brings up the whole stack incl. the frontend.
- Consul mesh on Compose (the earlier experiment): `make mesh-up` (+ `make mesh-resync` after recreations). See the repo root.
