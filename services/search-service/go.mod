module search-service

go 1.24

require (
	github.com/opensearch-project/opensearch-go/v2 v2.3.0
	github.com/segmentio/kafka-go v0.4.47
	google.golang.org/grpc v1.64.0
	google.golang.org/protobuf v1.34.1
)

require github.com/pierrec/lz4/v4 v4.1.15 // indirect

require (
	github.com/klauspost/compress v1.17.7 // indirect
	github.com/nikitashilov/microblog_grpc/proto v0.0.0
	golang.org/x/net v0.25.0 // indirect
	golang.org/x/sys v0.20.0 // indirect
	golang.org/x/text v0.15.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240318140521-94a12d6c2237 // indirect
)

replace github.com/nikitashilov/microblog_grpc/proto => ../../proto
