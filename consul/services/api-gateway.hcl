# Mesh registration for api-gateway: the app + its Connect sidecar proxy with a
# declared upstream to user-service. Envoy exposes that upstream as a local
# listener (127.0.0.1:5052); the gateway dials it instead of user-service:50052.
service {
  name    = "api-gateway"
  id      = "api-gateway"
  address = "@@ADDRESS@@"
  port    = 8080

  check {
    name     = "api-gateway HTTP health"
    http     = "http://@@ADDRESS@@:8080/health"
    interval = "10s"
    timeout  = "3s"
  }

  connect {
    sidecar_service {
      port = 21001
      proxy {
        upstreams = [
          {
            destination_name   = "user-service"
            local_bind_address = "127.0.0.1"
            local_bind_port    = 5052
          }
        ]
      }
    }
  }
}
