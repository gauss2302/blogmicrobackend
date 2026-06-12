# Tell the mesh that user-service speaks gRPC. This unlocks L7 features for it
# (gRPC-aware routing, retries, timeouts via service-router/splitter/resolver).
Kind     = "service-defaults"
Name     = "user-service"
Protocol = "grpc"
