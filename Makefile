dev:
	go run cmd/rest-api/main.go

grpc:
	go run cmd/grpc/main.go

buf:
	buf generate
