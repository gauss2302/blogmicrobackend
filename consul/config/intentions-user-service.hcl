# mTLS authorization for user-service: only api-gateway may call it; everything
# else is denied. Flip api-gateway's Action to "deny" (or `consul intention
# delete api-gateway user-service`) and the call fails with PermissionDenied even
# though the network path is open — that's identity-based authz, not firewall rules.
Kind = "service-intentions"
Name = "user-service"
Sources = [
  {
    Name   = "api-gateway"
    Action = "allow"
  },
  {
    Name   = "*"
    Action = "deny"
  }
]
