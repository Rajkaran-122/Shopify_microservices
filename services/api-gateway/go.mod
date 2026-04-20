module digital-metro/ecomm_java/services/api-gateway

go 1.21

require (
	google.golang.org/grpc v1.60.0
	google.golang.org/protobuf v1.31.0
)
EOF && cat > services/api-gateway/Dockerfile << 'EOF'
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o api-gateway .
FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/api-gateway .
EXPOSE 8080
CMD ["./api-gateway"]
