#!/usr/bin/env bash
# Bring up the whole stack on a local k3d cluster with the Linkerd mesh.
# Idempotent — safe to re-run. Requires: docker, k3d, kubectl, linkerd.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

CLUSTER="${CLUSTER:-blogmesh}"
GATEWAY_API_VERSION="${GATEWAY_API_VERSION:-v1.4.0}"
GO_SVCS="auth-service user-service post-service search-service notification-service api-gateway"
ALL_SVCS="$GO_SVCS frontend"

for t in docker k3d kubectl linkerd; do
  command -v "$t" >/dev/null 2>&1 || { echo "❌ missing tool: $t (brew install k3d linkerd; kubectl via Docker Desktop)"; exit 1; }
done

echo "==> [1/7] k3d cluster '$CLUSTER'"
if k3d cluster list "$CLUSTER" >/dev/null 2>&1; then
  echo "    already exists"
else
  k3d cluster create "$CLUSTER" --servers 1 \
    -p "8080:30080@server:0" -p "3000:30030@server:0" \
    --k3s-arg "--disable=traefik@server:0"
fi
kubectl config use-context "k3d-${CLUSTER}" >/dev/null

echo "==> [2/7] build app images (Go services build from repo root; frontend from ./frontend)"
for svc in $GO_SVCS; do
  docker build -q -t "blogmesh/${svc}:dev" -f "services/${svc}/Dockerfile" . >/dev/null
done
docker build -q -t "blogmesh/frontend:dev" ./frontend >/dev/null

echo "==> [3/7] import images into the cluster"
# shellcheck disable=SC2046
k3d image import -c "$CLUSTER" $(for s in $ALL_SVCS; do echo "blogmesh/${s}:dev"; done)

echo "==> [4/7] Gateway API CRDs + Linkerd control plane"
kubectl apply --server-side -f "https://github.com/kubernetes-sigs/gateway-api/releases/download/${GATEWAY_API_VERSION}/standard-install.yaml" >/dev/null
linkerd install --crds | kubectl apply -f - >/dev/null

# Long-lived identity certs (trust anchor 10y, issuer 1y) so the mesh doesn't
# expire after ~24h — Linkerd's auto-generated certs are short-lived. Needs the
# `step` CLI; falls back to auto certs (with a warning) if it's missing.
install_args=(--set proxyInit.runAsRoot=true)
if command -v step >/dev/null 2>&1; then
  CERTS="$(mktemp -d)"
  step certificate create root.linkerd.cluster.local "$CERTS/ca.crt" "$CERTS/ca.key" \
    --profile root-ca --no-password --insecure --not-after 87600h --force >/dev/null
  step certificate create identity.linkerd.cluster.local "$CERTS/issuer.crt" "$CERTS/issuer.key" \
    --profile intermediate-ca --not-after 8760h --no-password --insecure \
    --ca "$CERTS/ca.crt" --ca-key "$CERTS/ca.key" --force >/dev/null
  install_args+=(--identity-trust-anchors-file "$CERTS/ca.crt"
    --identity-issuer-certificate-file "$CERTS/issuer.crt"
    --identity-issuer-key-file "$CERTS/issuer.key")
else
  echo "    ! 'step' not found — using auto-generated certs (expire ~24h). 'brew install step' to avoid."
fi
linkerd install "${install_args[@]}" | kubectl apply -f - >/dev/null
kubectl wait --for=condition=available --all deployment -n linkerd --timeout=240s

echo "==> [5/7] Linkerd viz dashboard (optional — set SKIP_VIZ=1 to skip)"
if [ "${SKIP_VIZ:-0}" != "1" ]; then
  linkerd viz install | kubectl apply -f - >/dev/null
  kubectl wait --for=condition=available --all deployment -n linkerd-viz --timeout=240s
fi

echo "==> [6/7] namespace + secrets + infrastructure"
kubectl create namespace blogmesh --dry-run=client -o yaml | kubectl apply -f - >/dev/null
kubectl apply -f k8s/secret.yaml -f k8s/infra.yaml >/dev/null
kubectl wait --for=condition=available --all deployment -n blogmesh --timeout=300s

echo "==> [7/7] apps (auto-injected by Linkerd) + mesh policy"
kubectl apply -f k8s/apps.yaml >/dev/null
kubectl apply -f k8s/policy.yaml >/dev/null
kubectl wait --for=condition=available --all deployment -n blogmesh --timeout=300s

echo ""
echo "✅ up."
echo "   gateway:   http://localhost:8080/health"
echo "   frontend:  http://localhost:3000"
echo "   mesh UI:   linkerd viz dashboard"
echo "   mTLS:      linkerd viz edges deployment -n blogmesh"
