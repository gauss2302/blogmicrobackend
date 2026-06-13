#!/usr/bin/env bash
# Delete the local k3d cluster. Manifests under k8s/ are left untouched.
set -euo pipefail
CLUSTER="${CLUSTER:-blogmesh}"
k3d cluster delete "$CLUSTER"
echo "✅ cluster '$CLUSTER' deleted. Re-create with: bash k8s/up.sh"
