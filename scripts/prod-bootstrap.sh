#!/usr/bin/env bash
# One-shot production bootstrap. Run ON the prod node (k3s installed, kubectl
# pointing at it), from the repo root. Idempotent — safe to re-run.
#
# Installs: cert-manager, sealed-secrets controller, Gateway API CRDs, Linkerd
# (identity auto-rotated by cert-manager), then deploys the production overlay.
#
# Prereqs: k3s, kubectl, linkerd CLI. Domain + image org + ACME email replaced in
# the overlay, and k8s/overlays/production/sealed-secret.yaml REGENERATED with
# kubeseal (see docs/production.md). This script refuses to run otherwise.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

CERT_MANAGER_VERSION="${CERT_MANAGER_VERSION:-v1.16.3}"
SEALED_SECRETS_VERSION="${SEALED_SECRETS_VERSION:-v0.27.3}"
GATEWAY_API_VERSION="${GATEWAY_API_VERSION:-v1.4.0}"

for t in kubectl linkerd; do
  command -v "$t" >/dev/null 2>&1 || { echo "❌ missing tool: $t"; exit 1; }
done

# --- guard: refuse to deploy placeholder secrets ---
if grep -q "REGENERATE_WITH_kubeseal" k8s/overlays/production/sealed-secret.yaml; then
  echo "❌ k8s/overlays/production/sealed-secret.yaml is still the placeholder."
  echo "   Seal real secrets first (docs/production.md → Secrets). Aborting."
  exit 1
fi
# --- guard: refuse to deploy with the example domain ---
if grep -rq "blog.example.com" k8s/overlays/production/; then
  echo "❌ blog.example.com placeholder still present in k8s/overlays/production/."
  echo "   Set your real domain first. Aborting."
  exit 1
fi

echo "==> [1/6] cert-manager ${CERT_MANAGER_VERSION}"
kubectl apply -f "https://github.com/cert-manager/cert-manager/releases/download/${CERT_MANAGER_VERSION}/cert-manager.yaml"
kubectl -n cert-manager rollout status deploy/cert-manager-webhook --timeout=180s

echo "==> [2/6] sealed-secrets controller ${SEALED_SECRETS_VERSION}"
kubectl apply -f "https://github.com/bitnami-labs/sealed-secrets/releases/download/${SEALED_SECRETS_VERSION}/controller.yaml"
kubectl -n kube-system rollout status deploy/sealed-secrets-controller --timeout=180s

echo "==> [3/6] Linkerd identity (cert-manager-managed, auto-rotating)"
kubectl apply -f k8s/linkerd/identity-cert-manager.yaml
kubectl -n linkerd wait --for=condition=ready certificate/linkerd-identity-issuer --timeout=180s

echo "==> [4/6] Gateway API CRDs + Linkerd control plane"
kubectl apply --server-side -f "https://github.com/kubernetes-sigs/gateway-api/releases/download/${GATEWAY_API_VERSION}/standard-install.yaml"
linkerd install --crds | kubectl apply -f -
linkerd install --identity-external-issuer \
  --set identityTrustAnchorsPEM="$(kubectl -n linkerd get secret linkerd-trust-anchor -o jsonpath='{.data.tls\.crt}' | base64 -d)" \
  | kubectl apply -f -
linkerd check

echo "==> [5/6] deploy app stack (production overlay)"
kubectl apply -k k8s/overlays/production
kubectl -n blogmesh wait --for=condition=available --all deployment --timeout=600s

echo "==> [6/6] done"
kubectl -n blogmesh get ingress,certificate 2>/dev/null || true
echo ""
echo "✅ deployed. TLS cert is issued by cert-manager within ~1–2 min of DNS resolving."
echo "   Watch: kubectl -n blogmesh get certificate -w"
echo "   Verify mesh: linkerd viz check (if viz installed)"
