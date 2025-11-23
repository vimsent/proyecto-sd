gen-proto:
	protoc --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    api/proto/service.proto

docker-build:
	docker build --build-arg SERVICE_NAME=coordinator -t coordinator:latest .
	docker build --build-arg SERVICE_NAME=datanode -t datanode:latest .
	docker build --build-arg SERVICE_NAME=client -t client:latest .