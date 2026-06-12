#!/bin/sh
# Registers a service (+ its Connect sidecar) with Consul, then starts the Envoy
# sidecar proxy for it. Invoked as: sidecar-entrypoint.sh <service-name>
set -eu

SERVICE_NAME="${1:?usage: sidecar-entrypoint.sh <service-name>}"
CONSUL_HTTP="${CONSUL_HTTP_ADDR:-http://consul:8500}"
CONSUL_GRPC="${CONSUL_GRPC_ADDR:-consul:8502}"

# This sidecar shares the app container's network namespace, so `hostname -i`
# returns the app's routable IP on the Docker network. Envoy must bind/advertise
# an IP (a hostname yields "malformed IP address"), so we template it into the
# registration's address field.
APP_IP="$(hostname -i | awk '{print $1}')"
echo "[sidecar:${SERVICE_NAME}] app IP=${APP_IP}"

echo "[sidecar:${SERVICE_NAME}] waiting for Consul at ${CONSUL_HTTP} ..."
until consul info -http-addr="${CONSUL_HTTP}" >/dev/null 2>&1; do
  sleep 2
done

sed "s/@@ADDRESS@@/${APP_IP}/g" "/consul/services/${SERVICE_NAME}.hcl" > "/tmp/${SERVICE_NAME}.hcl"
echo "[sidecar:${SERVICE_NAME}] registering service definition (address ${APP_IP})"
consul services register -http-addr="${CONSUL_HTTP}" "/tmp/${SERVICE_NAME}.hcl"

echo "[sidecar:${SERVICE_NAME}] starting Envoy (xDS at ${CONSUL_GRPC})"
exec consul connect envoy \
  -sidecar-for="${SERVICE_NAME}" \
  -http-addr="${CONSUL_HTTP}" \
  -grpc-addr="${CONSUL_GRPC}" \
  -admin-bind="127.0.0.1:19000" \
  -- \
  -l info
