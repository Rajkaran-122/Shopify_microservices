.PHONY: help proto generate build run stop clean logs logs-user logs-product logs-order logs-gateway

help:
	@echo "Available commands:"
	@echo "  make proto         - Generate gRPC protobuf files"
	@echo "  make generate      - Run go generate"
	@echo "  make build         - Build all services"
	@echo "  make run           - Start all services with docker-compose"
	@echo "  make stop          - Stop all services"
	@echo "  make clean         - Clean build artifacts"

proto:
	@echo "Generating protobuf files..."
	protoc --go_out=. --go-grpc_out=. proto/user/user.proto
	protoc --go_out=. --go-grpc_out=. proto/product/product.proto
	protoc --go_out=. --go-grpc_out=. proto/order/order.proto
	protoc --go_out=. --go-grpc_out=. proto/notification/notification.proto

generate:
	go generate ./...

build:
	docker-compose build

run:
	docker-compose up -d

stop:
	docker-compose down

clean:
	docker-compose down -v
	rm -rf services/*/pb/

logs:
	docker-compose logs -f

logs-user:
	docker-compose logs -f user-service

logs-product:
	docker-compose logs -f product-service

logs-order:
	docker-compose logs -f order-service

logs-gateway:
	docker-compose logs -f api-gateway
