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
| Kafka | `user-service`, `post-service`, `search-service` |
| OpenSearch | `search-service` |
| Postgres databases | owning service only |
| Redis | `api-gateway`, `auth-service` |

Do not publish internal gRPC ports or non-gateway HTTP ports on a public load balancer.

## Internal HTTP Trust

Non-gateway HTTP services use internal headers such as `X-User-ID` on legacy/private routes. In production, set:

```env
INTERNAL_HTTP_TRUST_MODE=private_network
```

This is an explicit declaration that those HTTP listeners are reachable only from trusted internal infrastructure. Use `disabled` once the legacy/private HTTP surface is removed or blocked by deployment configuration.

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
