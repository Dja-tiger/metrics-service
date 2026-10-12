package proto

//go:generate protoc -I ../.. --go_out=../.. --go_opt=paths=source_relative --go_opt=default_api_level=API_OPAQUE --go-grpc_out=../.. --go-grpc_opt=paths=source_relative internal/proto/metrics.proto
