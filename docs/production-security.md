# Production Security Notes

This stack is designed with `api-gateway` as the only public entry point. All service-to-service gRPC traffic must run inside a trusted production boundary.

## Service Transport Modes

Set `SERVICE_TRANSPORT_SECURITY` in every production deployment:

- `mesh` - gRPC remains plaintext inside the application process, but traffic is protected by infrastructure-level mTLS such as a service mesh, sidecar proxy, or equivalent private service network.
- `app_mtls` - Go gRPC clients and servers use the `GRPC_TLS_*` certificate settings directly. The mode is mutual TLS only: the services refuse to start unless `GRPC_TLS_ENABLED=true`, `GRPC_TLS_CA_FILE`, `GRPC_TLS_CERT_FILE` and `GRPC_TLS_KEY_FILE` are set, and the gRPC servers also need `GRPC_TLS_REQUIRE_CLIENT_CERT=true`. The gateway is only a client and presents its certificate as the client certificate.
- `insecure_dev` - local development only. This mode is rejected when `ENVIRONMENT=production`.

For this project, production should use `SERVICE_TRANSPORT_SECURITY=mesh` unless the deployment intentionally moves mTLS into the Go processes.

## Who May Reach The Gateway And The Frontend

In the production overlay the gateway trusts `X-Forwarded-For` from its own Linkerd proxy and the frontend BFF trusts one proxy hop, so that the rate limiter sees the real client behind Traefik. Every in-cluster caller arrives through the same proxy. `k8s/overlays/production/networkpolicy.yaml` therefore admits only Traefik and the frontend to `api-gateway:8080`, and only Traefik to `frontend:3000` (plus linkerd-viz to the proxy's metrics and tap ports); without it any pod could choose the address it is rate-limited under. Traefik is selected by namespace `kube-system` and label `app.kubernetes.io/name: traefik` (the chart bundled with k3s). A different ingress controller, or a Prometheus that scrapes the gateway, has to be added to that file.

## Required Network Boundary

Production must enforce these access rules at the ingress, service mesh, firewall, or Kubernetes NetworkPolicy layer:

| Target | Allowed callers |
| --- | --- |
| `api-gateway` HTTP | public ingress only |
| `auth-service` gRPC | `api-gateway` |
| `auth-service` HTTP | private network only |
| `user-service` gRPC | `api-gateway`, `auth-service`, `search-service` |
| `user-service` HTTP | private network only |
| `post-service` gRPC | `api-gateway` |
| `post-service` HTTP | private network only |
| `search-service` gRPC | `api-gateway` |
| `notification-service` HTTP | private network only |
| RabbitMQ | `post-service`, `notification-service` |
| Kafka | `post-service`, `user-service`, `search-service` |
| OpenSearch | `search-service` |
| Postgres databases | owning service only, plus the backup CronJob |
| Redis | `api-gateway`, `auth-service`, `user-service`, `post-service`, `notification-service` |

Do not publish internal gRPC ports or non-gateway HTTP ports on a public load balancer.

### How the data tier is enforced

The service-to-service rows are enforced by Linkerd `Server` + `AuthorizationPolicy` (`k8s/base/policy.yaml`), which authenticate callers by workload identity.

That mechanism cannot cover the data tier: the infra pods are not mesh-injected, and every app pod skips the proxy for those ports (`config.linkerd.io/skip-outbound-ports` in `k8s/base/apps.yaml`). The datastore rows are therefore enforced by Kubernetes NetworkPolicy (`k8s/base/networkpolicy.yaml`) — one policy per component, each acting as both the default-deny and the sole allow-list. Adding a component or a caller means adding it there, or the connection is refused.

**Authentication:** OpenSearch requires HTTP basic auth. The node admin password is `OPENSEARCH_PASSWORD`; search-service logs in as a separate user (`OPENSEARCH_SEARCH_USERNAME` / `OPENSEARCH_SEARCH_PASSWORD`) whose role covers only the `users` and `posts` indices: it can create them, read and write documents, and nothing else. Kafka's client listener is SASL/PLAIN with two users: `admin` (`KAFKA_ADMIN_PASSWORD`) is the broker's own identity, and the services share `app` (`KAFKA_SASL_USERNAME` / `KAFKA_SASL_PASSWORD`), which ACLs limit to topics `search.*` and consumer groups `search-*`. Both the OpenSearch user and the ACLs are created by the infrastructure containers themselves on every start. The same step removes the demo users the OpenSearch image ships with (their password is their name, and `readall` can read every index); the reserved `kibanaserver` cannot be removed from a data volume that already has it, only by recreating the volume. All three services share the one Kafka user, and the passwords travel in plaintext inside the cluster network (SASL_PLAINTEXT, HTTP): neither is addressed here. NetworkPolicy still limits which pods can open the ports. A compromise of a workload that holds those credentials can still use the index or the topics that workload is allowed to use.

## Internal HTTP Trust

The non-gateway HTTP APIs of `user-service`, `post-service` and `notification-service` authenticate callers from the `Authorization: Bearer <access token>` header, verify the signature with the `JWT_SECRET` shared with `auth-service`, and reject the token when `auth:blacklist:<token>` exists in Redis. Revoking all of a user's sessions (logout, deleting one's own account, or a reused refresh token) deletes every token of the user from auth-service's store, which the gateway checks, and blacklists each of the user's access tokens still in that store for the rest of its lifetime, so they stop working on these APIs at once as well. These services must read the same Redis database as auth-service: Compose and `k8s/base/apps.yaml` pass all four the same `REDIS_DB`. A refresh revokes nothing about the access token it replaces: that token stays valid until it expires, at the gateway too. Identity is never taken from an `X-User-ID` header: that fallback exists only in `insecure_dev`, which config validation rejects in production.

`Follow` and `Unfollow` on user-service gRPC do the same check on the access token carried in gRPC metadata `authorization`. The `follower_id` in the request must equal the user id in that token. The gateway sets both from the caller it already authenticated.

In production, set:

```env
INTERNAL_HTTP_TRUST_MODE=private_network
```

This is an explicit declaration that those HTTP listeners are reachable only from trusted internal infrastructure. Use `disabled` once the legacy/private HTTP surface is removed or blocked by deployment configuration. Neither mode weakens token verification: a presented token is always signature-checked, and identity is never taken from a header. Routes that are deliberately public (post and user listings, search, stats) stay public in both modes; post-service's `/stats` merely adds the caller's own post count when a valid token is present.

Because every service above verifies tokens minted by `auth-service`, all four must be given the same `JWT_SECRET`; Compose passes one root-level value to all of them so they cannot drift.

`auth-service` writes `JWT_ISSUER` (default `auth-service`) into the `iss` claim and rejects a token with any other issuer, including at the gateway, which validates through it. Changing `JWT_ISSUER` therefore ends every session: all issued access and refresh tokens stop working and users sign in again. The internal HTTP APIs do not check `iss`: only holders of `JWT_SECRET` can sign, so the signature already bounds who issues tokens.

## Secrets And Defaults

The Compose file intentionally requires secret-bearing variables instead of falling back to known passwords. Keep real values in your deployment secret manager or an ignored local `.env` file. Use `.env.example` only as a template.

Required production secrets include:

- `POSTGRES_USER_PASSWORD`
- `POSTGRES_POST_PASSWORD`
- `POSTGRES_NOTIFICATION_PASSWORD`
- `REDIS_PASSWORD`
- `RABBITMQ_PASSWORD`
- `GOOGLE_CLIENT_SECRET`
- `JWT_SECRET`
- `OPENSEARCH_PASSWORD`
- `OPENSEARCH_SEARCH_PASSWORD`
- `KAFKA_ADMIN_PASSWORD`
- `KAFKA_SASL_PASSWORD`

## Sign-In Protection

- **Password guessing per account.** auth-service counts failed password sign-ins per email (`auth:login_fail:<sha256 of the email>`, so Redis holds no addresses). Ten failures within 15 minutes of the first refuse password sign-in for that email until the window ends, with `TOO_MANY_LOGIN_ATTEMPTS` (gRPC `ResourceExhausted`); a successful sign-in resets the count. Unknown emails are counted the same way, so a lockout does not reveal whether an account exists. The cost: anyone who knows an email can block password sign-in for it for 15 minutes with ten wrong attempts. Google sign-in keeps working for that user. The gateway's per-IP limiter on auth routes applies on top. If Redis is unavailable, password sign-in fails with 503 rather than skipping the check.
- **Google domains.** `GOOGLE_ALLOWED_DOMAINS` is matched against the `hd` (Google Workspace domain) claim of the Google profile, not the email suffix: a personal Google account can be registered with any address, a company one included, and has no `hd`. The check runs before the account is created, so it limits Google registration too. It does not apply to email/password registration.
- **Token store failures.** When Redis cannot be read while validating a token, auth-service answers `Unavailable` (503 at the gateway), not 401, so clients retry instead of dropping the session.

## gRPC Identity Model

The protobuf APIs carry identities in message fields such as `user_id`, `actor_id`, `follower_id`, and `requesting_user_id`. `Follow` and `Unfollow` do not trust `follower_id` by itself: the caller must present the same access token in gRPC metadata, and that token's user id must match. Other identity fields are safe only when the production network boundary guarantees that arbitrary clients cannot connect directly to service gRPC ports.

If that guarantee cannot be made for the remaining methods, authenticate them the same way before exposing the services.
