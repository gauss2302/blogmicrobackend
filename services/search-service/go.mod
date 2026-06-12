module search-service

go 1.24

require (
	github.com/opensearch-project/opensearch-go/v2 v2.3.0
	github.com/prometheus/client_golang v1.20.5
	github.com/segmentio/kafka-go v0.4.47
	google.golang.org/grpc v1.73.0
	google.golang.org/protobuf v1.36.6
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/pierrec/lz4/v4 v4.1.15 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.55.0 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
)

require (
	github.com/klauspost/compress v1.17.9 // indirect
	github.com/nikitashilov/microblog_grpc/proto v0.0.0
	golang.org/x/net v0.38.0 // indirect
	golang.org/x/sys v0.31.0 // indirect
	golang.org/x/text v0.23.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250324211829-b45e905df463 // indirect
)

replace github.com/nikitashilov/microblog_grpc/proto => ../../proto
