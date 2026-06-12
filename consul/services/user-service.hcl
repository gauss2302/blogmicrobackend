# Mesh registration for user-service: the app + its Connect sidecar proxy.
# Registered by the sidecar's entrypoint via `consul services register`.
service {
  name    = "user-service"
  id      = "user-service"
  address = "@@ADDRESS@@"
  port    = 50052

  # gRPC health protocol (grpc.health.v1.Health) implemented in main.go.
  checks = [
    {
      name                              = "user-service gRPC health"
      grpc                              = "@@ADDRESS@@:50052"
      grpc_use_tls                      = false
      interval                          = "10s"
      timeout                           = "3s"
      deregister_critical_service_after = "1m"
    }
  ]

  connect {
    sidecar_service {
      port = 21000
    }
  }
}
