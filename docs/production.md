# Production deployment

This is the single source of truth for taking the stack to production. The target
is intentionally **simple**: one VPS running **k3s**, with Linkerd (mesh + mTLS),
Traefik ingress + Let's Encrypt TLS (cert-manager), persistent volumes, and images
pulled from GitHub Container Registry. Deploy is one command after a one-time setup.

```
Internet ──TLS──► Traefik Ingress ──► frontend (/)  +  api-gateway (/api/v1)
   (cert-manager / Let's Encrypt)          │
                                           ├─ Linkerd mesh (mTLS + authz) between services
                                           ├─ PVCs: postgres ×3, opensearch, redis (+ daily pg backup)
                                           └─ ephemeral: kafka, rabbitmq (transient event pipes)
```

Everything is Kustomize: **`k8s/overlays/local`** (k3d dev, unchanged) and
**`k8s/overlays/production`** (this guide). Preview either with `make render-prod`.

---

## 0. Prerequisites

**A VPS** — 1 node, ≥ 4 vCPU / 8 GB RAM / 40 GB disk (opensearch + kafka are the
heavy tenants). Hetzner CX32 / DO 8 GB are fine. Ubuntu 22.04+.

**A domain** with an `A` record you can point at the VPS IP.

**Local tools** (your laptop): `kubectl`, `linkerd`, `kubeseal`, `git`.
```bash
brew install kubectl linkerd2 kubeseal   # macOS
```

---

## 1. Provision the node (k3s)

SSH to the VPS and install k3s (ships Traefik ingress + local-path storage):
```bash
curl -sfL https://get.k3s.io | sh -
```
Copy the kubeconfig to your laptop and point it at the public IP:
```bash
scp root@VPS_IP:/etc/rancher/k3s/k3s.yaml ./k3s.yaml
sed -i '' "s/127.0.0.1/VPS_IP/" ./k3s.yaml      # macOS sed
export KUBECONFIG="$PWD/k3s.yaml"
kubectl get nodes                                # should be Ready
```
Open the firewall to **80/443 only** (Traefik). Everything else stays internal.

---

## 2. DNS

Point your domain at the node: `A  blog.yourdomain.com → VPS_IP`. Verify it
resolves before deploying (cert issuance needs it):
```bash
dig +short blog.yourdomain.com
```

---

## 3. Fill in the placeholders

All placeholders live in `k8s/overlays/production/` and are guarded by the
bootstrap script (it refuses to deploy if any remain). Replace:

| Placeholder | Where | Set to |
|---|---|---|
| `blog.example.com` | `ingress.yaml`, `patches/app-config.yaml` | your domain |
| `admin@example.com` | `cluster-issuer.yaml` | your email (ACME) |
| `ghcr.io/devshilov` | `kustomization.yaml` (`images:`) | `ghcr.io/<your-gh-owner>` |
| image tag `v0.1.0` | `kustomization.yaml` (`images:`) | the release tag you push in step 4 |

```bash
cd k8s/overlays/production
grep -rl 'blog.example.com\|devshilov\|admin@example.com' . | xargs sed -i '' \
  -e 's/blog.example.com/blog.yourdomain.com/g' \
  -e 's#ghcr.io/devshilov#ghcr.io/your-gh-owner#g' \
  -e 's/admin@example.com/you@yourdomain.com/g'
```

---

## 4. Build + push images

Tag a release; CI (`.github/workflows/build-push.yml`) builds all 7 images and
pushes them to `ghcr.io/<owner>/<svc>:<tag>` + `:latest`:
```bash
git tag v0.1.0 && git push origin v0.1.0
```
Then make the packages **public** (GitHub → your profile → Packages → each package
→ Package settings → Change visibility → Public) — simplest. *Private alternative:*
create an image pull secret and attach it to the namespace default ServiceAccount:
```bash
kubectl -n blogmesh create secret docker-registry ghcr \
  --docker-server=ghcr.io --docker-username=<owner> --docker-password=<PAT-with-read:packages>
kubectl -n blogmesh patch serviceaccount default \
  -p '{"imagePullSecrets":[{"name":"ghcr"}]}'
```

Set the matching tag in `kustomization.yaml` (`newTag: v0.1.0`).

---

## 5. Secrets (sealed, safe to commit)

Author a **plaintext** Secret locally — **never commit it**:
```bash
cat > /tmp/blogmesh.secret.yaml <<'EOF'
apiVersion: v1
kind: Secret
metadata: { name: blogmesh-secrets, namespace: blogmesh }
type: Opaque
stringData:
  POSTGRES_USER_PASSWORD: "<strong-random>"
  POSTGRES_POST_PASSWORD: "<strong-random>"
  POSTGRES_NOTIFICATION_PASSWORD: "<strong-random>"
  DATABASE_URL_USER: "postgres://postgres:<pw>@postgres-user:5432/userdb?sslmode=disable"
  DATABASE_URL_POST: "postgres://postgres:<pw>@postgres-post:5432/postdb?sslmode=disable"
  DATABASE_URL_NOTIFICATION: "postgres://postgres:<pw>@postgres-notification:5432/notificationdb?sslmode=disable"
  REDIS_PASSWORD: "<strong-random>"
  RABBITMQ_USER: "blog"
  RABBITMQ_PASSWORD: "<strong-random>"
  RABBITMQ_VHOST: "blog"
  RABBITMQ_URL: "amqp://blog:<pw>@rabbitmq:5672/blog"
  JWT_SECRET: "<>= 32 random chars>"
  GOOGLE_CLIENT_ID: "<your-oauth-client-id>"
  GOOGLE_CLIENT_SECRET: "<your-oauth-client-secret>"
EOF
```
Generate randoms with `openssl rand -base64 24`. The DB passwords must match
between `POSTGRES_*_PASSWORD` and the corresponding `DATABASE_URL_*`.

> The sealed-secrets **controller must be running** before you can seal (step 6
> installs it). On first run: do step 6 up to the controller, then come back here.

Seal it (only your cluster can decrypt the output — safe to commit):
```bash
kubeseal --controller-name sealed-secrets-controller --controller-namespace kube-system \
  --format yaml < /tmp/blogmesh.secret.yaml > k8s/overlays/production/sealed-secret.yaml
rm /tmp/blogmesh.secret.yaml
```

In your **Google Cloud OAuth client**, add the authorized redirect URI:
`https://blog.yourdomain.com/api/v1/auth/google/callback`.

---

## 6. Bootstrap + deploy (one command)

From your laptop with `KUBECONFIG` set:
```bash
make prod-bootstrap
```
This installs cert-manager, the sealed-secrets controller, the auto-rotating
Linkerd identity (cert-manager-managed — no manual cert expiry), Gateway API CRDs,
Linkerd, then `kubectl apply -k k8s/overlays/production`. It is idempotent and
refuses to run while placeholders/placeholder-secrets remain.

> First run ordering: if the sealed-secrets controller isn't up yet when you reach
> step 5, run `make prod-bootstrap` once (it installs the controller and will stop
> at the secret guard), then do step 5, then re-run `make prod-bootstrap`.

Watch the TLS cert get issued (1–2 min after DNS resolves):
```bash
kubectl -n blogmesh get certificate -w
```

---

## 7. Verify

```bash
kubectl -n blogmesh get pods                          # all Running, 2/2 (app+proxy)
linkerd viz check                                     # if viz installed
curl -fsS https://blog.yourdomain.com/api/v1/health   # gateway via TLS
open https://blog.yourdomain.com                      # the app
```
Smoke test: register → create a post → search finds it. mTLS + authz are enforced
by the same Linkerd policies as local (gateway-only access to each backend).

---

## Day-2 operations

**Redeploy** (after code/manifest changes): push a new tag, bump `newTag` in
`kustomization.yaml`, then `make prod-deploy`.

**Backups** — a CronJob runs `pg_dump` of all three DBs nightly to the
`postgres-backups` PVC (14-day retention). **Node-local by default** — for
off-site durability add an S3 upload step (mc/rclone) to `backup-cronjob.yaml`.
Restore:
```bash
kubectl -n blogmesh exec -it deploy/postgres-post -- \
  sh -c 'zcat /backups/post-<ts>.sql.gz | psql -U postgres -d postdb'   # via a backups mount
```

**Linkerd certs** rotate automatically (cert-manager). No annual manual step.
`linkerd check --proxy` to confirm.

**Scaling** — bump `replicas:` in the prod kustomization (api-gateway/frontend are
2 by default). Backends/infra are single-instance on one node; for HA move to
multiple nodes or managed datastores.

**Logs / mesh** — `kubectl -n blogmesh logs deploy/<svc> -c <svc>`;
`linkerd viz stat deploy -n blogmesh`.

---

## Hardening checklist (deferred / next steps)

These are deliberately left for when you scale past a single-node blog:

- **Datastore TLS** — app↔postgres uses `sslmode=disable` (in-cluster, single
  node, plaintext bypasses the mesh by design). Enable postgres TLS or move to a
  managed DB for cross-node encryption.
- **`readOnlyRootFilesystem`** — set on app containers (`securityContext`) after
  validating each service's writable paths.
- **Off-site backups** — wire the pg backup CronJob to object storage.
- **HA** — multi-node k3s + PodDisruptionBudgets + anti-affinity, or managed
  Postgres/Redis/Kafka/OpenSearch.
- **Meshed ingress** — inject Linkerd into Traefik for edge-to-app mTLS.
