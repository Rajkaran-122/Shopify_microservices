module digital-metro/ecomm_java/services/notification-service

go 1.21

require (
	google.golang.org/grpc v1.60.0
	google.golang.org/protobuf v1.31.0
)
EOF && cat > services/notification-service/Dockerfile << 'EOF'
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o notification-service .
FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/notification-service .
EXPOSE 50054
CMD ["./notification-service"]
