# blogmicrobackend

Polyglot template for a small blog platform: a Go microservice backend behind one HTTP API gateway, plus a Next.js frontend. The public entry point is `api-gateway`. Services talk to each other over gRPC. Postgres is one database per service. Post lifecycle events go through RabbitMQ. Search indexing goes through Kafka into OpenSearch.

The repository owner is [gauss2302/blogmicrobackend](https://github.com/gauss2302/blogmicrobackend). Container images are published to `ghcr.io/gauss2302/<service>`. Forks should replace that owner in `k8s/overlays/production/kustomization.yaml` before the first deploy. See `docs/production.md`.

## Layout

- `services/` — `api-gateway`, `auth-service`, `user-service`, `post-service`, `notification-service`, `search-service`. Each is its own Go module.
- `proto/` — shared gRPC contracts, module `github.com/gauss2302/blogmicrobackend/proto`.
- `frontend/` — Next.js app. Package manager is bun.
- `k8s/` — Kustomize base, local overlay, and production overlay (k3s, Linkerd, Traefik).
- `docs/production.md` — production deploy. `docs/production-security.md` — the trust boundary.

## Local stack

Copy `.env.example` to `.env` and replace every `replace-with-*` value. The example sets `ENVIRONMENT=development` so Compose can boot. Setting `ENVIRONMENT=production` while a placeholder secret is still present makes the services refuse to start.

```bash
make up-d
```

API gateway listens on `:8080`. The frontend dev server defaults to `:3000`.

To iterate on one service against Compose infrastructure:

```bash
make infra-up
cd services/<name> && go run .
```

Compose hostnames (`postgres_user`, `redis`, `kafka`) only resolve inside the Docker network. Override them when the process runs on the host, for example `REDIS_URL=localhost:6379`.

## Checks

```bash
cd services/<name> && go test ./...
cd frontend && bun install && bun run lint && bun run typecheck && bun run test
```

GitHub Actions runs the same checks on every push (`.github/workflows/verify.yml`) and publishes images when a `vX.Y.Z` tag is pushed.

## Production

One k3s node, Linkerd for service-to-service mTLS, Traefik and cert-manager for public TLS. The production overlay patches `ENVIRONMENT` to `production` and expects real secrets through a SealedSecret. OpenSearch authentication and Kafka SASL are not enabled yet: NetworkPolicy limits who can reach those ports, and `docs/production-security.md` records that residual risk.
