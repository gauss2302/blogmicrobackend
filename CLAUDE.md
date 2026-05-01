# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository layout

Polyglot monorepo: a Go microservice backend plus a Next.js frontend.

- `proto/` — shared gRPC contracts (`auth/v1`, `user/v1`, `post/v1`, `search/v1`). Its own Go module (`github.com/nikitashilov/microblog_grpc/proto`) consumed by every service via a `replace` directive in their `go.mod` (e.g. `replace github.com/nikitashilov/microblog_grpc/proto => ../../proto`). Generated `.pb.go` / `_grpc.pb.go` files are committed.
- `services/<name>/` — six independent Go modules (`api-gateway`, `auth-service`, `user-service`, `post-service`, `notification-service`, `search-service`). Each has its own `go.mod`, `Dockerfile`, `main.go`.
- `frontend/` — Next.js 16 App Router app (React 19, Tailwind 4, TanStack Query, Zustand). Package manager is **bun** (`bun.lock`).
- `docker-compose.yml` + `Makefile` — entire stack orchestration. Only `api-gateway` exposes a host port (`:8080`); all other services live on internal Docker networks (`internal_net`, `edge_net`).
- `scripts/postgres-init-*.sql` — bootstrap SQL mounted into the per-service Postgres containers.
- `config/rabbitmq.conf` — broker config for the notification flow.
- `docs/` — design notes (auth flow, search rollout). Read these before changing auth or search behavior.

## Common commands

### Stack (Docker Compose, via `make`)
- `make up-d` / `make down` — start/stop everything detached.
- `make infra-up` — start only infra (redis, postgres x3, rabbitmq, opensearch, kafka). Used when running services locally with `go run`.
- `make app-up` / `make app-down` — start/stop only application services.
- `make logs-svc SVC=auth-service` — follow logs for one service.
- `make shell SVC=api-gateway` — shell into a container.
- `make build` / `make build-no-cache` — (re)build images.
- `make clean` — `down -v --remove-orphans` (drops volumes; destroys DB data).

### Go services
Each service is a separate module. Always `cd services/<name>` first.
- Build: `go build ./...`
- Test (whole module): `go test ./...`
- Test single package: `go test ./internal/application/services/...`
- Test single test: `go test ./internal/... -run TestName`
- After changing dependencies in any service: `go mod tidy` inside that service directory.
- The `proto` module also has tests: `cd proto && go test ./...`.

### Local hybrid run (recommended for backend dev)
1. `make infra-up`
2. `cd services/<name> && go run .` per service you're iterating on.
3. The Compose env vars (Postgres URL, Redis URL, gRPC addresses) assume Docker DNS hostnames (`postgres_user`, `redis`, `auth-service`, etc.). Override with env vars when running outside the network — e.g. `REDIS_URL=localhost:6379 USER_SERVICE_GRPC_ADDR=localhost:50052 go run .`.

### Frontend
From `frontend/`:
- `bun install`
- `bun run dev` — Next dev server (defaults to :3000).
- `bun run build` — production build (uses webpack, not turbopack — see `package.json`).
- `bun run lint` / `bun run typecheck`.

## Architecture

### Service topology
The API Gateway is the only public entry point. It speaks **HTTP/REST to clients** and **gRPC to backend services**.

```
client ─HTTP─► api-gateway :8080
                  │
                  ├─gRPC─► auth-service     :50051 (+ HTTP :8081)
                  ├─gRPC─► user-service     :50052 (+ HTTP :8082)
                  ├─gRPC─► post-service     :50053 (+ HTTP :8083)
                  └─gRPC─► search-service   :50054
                                  ▲
                  Redis ◄─── auth-service (tokens, blacklist, OAuth state, auth_code)
                  Postgres (per-service): postgres_user, postgres_post, postgres_notification
                  RabbitMQ ── post-service publishes ──► notification-service consumes
                  Kafka ── user/post-service publish ──► search-service consumes ──► OpenSearch
```

Notes:
- `auth-service`, `user-service`, `post-service`, `search-service` each run a gRPC server. `auth/user/post` additionally expose an HTTP server (mostly health/legacy). Gateway-to-service traffic is gRPC only.
- `notification-service` has no gRPC; it consumes from RabbitMQ and exposes an HTTP API for reading notifications.
- `search-service` has no database of its own — it reads from OpenSearch (queried) and Kafka (indexed) and falls back to `user-service` gRPC for follow-state demotion.

### Per-service code structure (DDD-ish)
Most services follow:
```
internal/
  application/   # use cases, services, dto, errors
  domain/        # entities, repository interfaces
  infrastructure/  # postgres, redis, rabbitmq, kafka, opensearch, oauth — concrete impls
  interfaces/    # grpc/, http/, validators — inbound adapters
  config/        # env loading
clients/         # outbound gRPC clients (auth-service uses this for user-service)
pkg/logger/      # local logger (each service has its own copy)
```
Exceptions: `api-gateway` is flatter (`clients/`, `handlers/`, `middleware/`, `routes/`, `models/`); `notification-service` uses `interface/` (singular) and puts repo + connection at the top of `infrastructure/`; `search-service` has no `domain/` (no persistent entities).

### Auth model (see `docs/auth-user-management-and-verification.md`)
- JWTs split by type: `access` (short TTL) and `refresh` (long TTL). Both stored in Redis; logout/refresh blacklists the prior token.
- Email/password: gateway → auth-service gRPC. Auth-service calls user-service to create/validate credentials (bcrypt in user-service).
- Google OAuth: secure auth-code exchange. Web is plain; **mobile requires PKCE**. Flow: `GET /api/v1/auth/google` → Google → `GET /api/v1/auth/google/callback` (issues a 5-min `auth_code` in Redis, redirects to client) → `POST /api/v1/auth/exchange` (returns JWT pair). State and auth_code use `GETDEL` for one-shot semantics.
- Authorization on user mutations: gateway extracts `userID` from the access token and passes it as `actor_id` in gRPC; user-service enforces `actor_id == id` for update/delete.
- Refresh token can be carried in HttpOnly cookie (`AUTH_REFRESH_TOKEN_COOKIE=true`) or JSON body.
- **Caveat**: `DeleteUserTokens` uses `KEYS auth:*:*` — O(N), do not assume it scales.

### Routing (gateway)
`services/api-gateway/internal/routes/routes.go` is the source of truth for the public API surface:
- `/api/v1/auth/*` — register/login/google/callback/exchange/refresh (public) + logout/validate (protected).
- `/api/v1/public/users/*` and `/api/v1/public/posts/*` — public reads with `OptionalAuthMiddleware`.
- `/api/v1/users`, `/api/v1/posts`, `/api/v1/search` — protected by `AuthMiddleware`. Includes follow graph (`/users/:id/follow`, `/followers`, `/following`).

### Search rollout (see `docs/search-rollout.md`)
Phased: deploy follow schema → deploy search-service + OpenSearch/Kafka → enable Kafka publishing from user/post-services and backfill → enable gateway `/api/v1/search` and frontend Discover. OpenSearch outage degrades to partial results, not a top-level error.

### Database migrations
Each service runs its own migrations on startup from `internal/infrastructure/.../migrations.go`, driven by `DB_MIGRATION_PATH` (defaults to `./migrations`). The `scripts/postgres-init-*.sql` files only bootstrap the database/role at first container start.

### Configuration
All runtime config is env-based. Each service has `internal/config/` and reads from a `.env` file (`./services/<name>/.env`, mounted via `env_file:` in compose) plus environment overrides. Common knobs: `LOG_LEVEL`, `ENVIRONMENT`, `GRPC_TLS_*`, `GRPC_REFLECTION_ENABLED`, per-service `*_GRPC_ADDR`. mTLS is supported but off by default.

## Conventions to preserve

- **Don't bypass the proto module's replace directive** — when adding a new service, copy the pattern: separate `go.mod`, `replace github.com/nikitashilov/microblog_grpc/proto => ../../proto`. Dockerfiles build from the **repo root** so they can `COPY proto`; mirror the existing `services/<svc>/Dockerfile` layout.
- **Gateway ↔ services is gRPC, not HTTP.** Add new cross-service calls via the proto contracts, regenerate, and wire a client in `services/api-gateway/internal/clients/`.
- **Authorization belongs on the receiving service** (user-service checks `actor_id`), not only on the gateway.
- **One Postgres per service** — don't add cross-service joins; communicate via gRPC or events.
- **Events**: post lifecycle → RabbitMQ (`post.created/updated/deleted`) consumed by notification-service. Search indexing → Kafka topics `search.users` / `search.posts` consumed by search-service. Don't conflate the two buses.
