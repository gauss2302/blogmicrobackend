# Production Security Notes

This stack is designed with `api-gateway` as the only public entry point. All service-to-service gRPC traffic must run inside a trusted production boundary.

## Service Transport Modes

Set `SERVICE_TRANSPORT_SECURITY` in every production deployment:

- `mesh` - gRPC remains plaintext inside the application process, but traffic is protected by infrastructure-level mTLS such as a service mesh, sidecar proxy, or equivalent private service network.
- `app_mtls` - Go gRPC clients and servers use the `GRPC_TLS_*` certificate settings directly.
- `insecure_dev` - local development only. This mode is rejected when `ENVIRONMENT=production`.

For this project, production should use `SERVICE_TRANSPORT_SECURITY=mesh` unless the deployment intentionally moves mTLS into the Go processes.

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
| Kafka | `post-service`, `search-service` |
| OpenSearch | `search-service` |
| Postgres databases | owning service only, plus the backup CronJob |
| Redis | `api-gateway`, `auth-service` |

Do not publish internal gRPC ports or non-gateway HTTP ports on a public load balancer.

### How the data tier is enforced

The service-to-service rows are enforced by Linkerd `Server` + `AuthorizationPolicy` (`k8s/base/policy.yaml`), which authenticate callers by workload identity.

That mechanism cannot cover the data tier: the infra pods are not mesh-injected, and every app pod skips the proxy for those ports (`config.linkerd.io/skip-outbound-ports` in `k8s/base/apps.yaml`). The datastore rows are therefore enforced by Kubernetes NetworkPolicy (`k8s/base/networkpolicy.yaml`) — one policy per component, each acting as both the default-deny and the sole allow-list. Adding a component or a caller means adding it there, or the connection is refused.

**Residual risk:** OpenSearch still runs with its security plugin disabled and Kafka with a PLAINTEXT listener, so neither authenticates clients. The NetworkPolicies are what keep the set of workloads that can reach them down to one or two; a compromise of `search-service` (or `post-service`, for Kafka) still yields unrestricted access to the index or the topics. Enabling OpenSearch authentication and Kafka SASL remains open work — track it before treating this tier as hardened.

## Internal HTTP Trust

The non-gateway HTTP APIs of `user-service`, `post-service` and `notification-service` authenticate callers from the `Authorization: Bearer <access token>` header and verify the signature with the `JWT_SECRET` shared with `auth-service`. Identity is never taken from an `X-User-ID` header: that fallback exists only in `insecure_dev`, which config validation rejects in production.

In production, set:

```env
INTERNAL_HTTP_TRUST_MODE=private_network
```

This is an explicit declaration that those HTTP listeners are reachable only from trusted internal infrastructure. Use `disabled` once the legacy/private HTTP surface is removed or blocked by deployment configuration. Neither mode weakens token verification: a presented token is always signature-checked, and identity is never taken from a header. Routes that are deliberately public (post and user listings, search, stats) stay public in both modes; post-service's `/stats` merely adds the caller's own post count when a valid token is present.

Because every service above verifies tokens minted by `auth-service`, all four must be given the same `JWT_SECRET`; Compose passes one root-level value to all of them so they cannot drift.

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

## gRPC Identity Model

The protobuf APIs carry identities in message fields such as `user_id`, `actor_id`, `follower_id`, and `requesting_user_id`. That is safe only when the production network boundary guarantees that arbitrary clients cannot connect directly to service gRPC ports.

If this guarantee cannot be made, add message-level authentication using gRPC metadata and service-side principal validation before exposing the services.
