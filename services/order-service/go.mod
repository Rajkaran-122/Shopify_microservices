module digital-metro/ecomm_java/services/order-service

go 1.21

require (
	google.golang.org/grpc v1.60.0
	google.golang.org/protobuf v1.31.0
)
EOF && cat > services/order-service/Dockerfile << 'EOF'
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o order-service .
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/order-service .
EXPOSE 50053
CMD ["./order-service"]
