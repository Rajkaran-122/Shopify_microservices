# syntax=docker/dockerfile:1

# Build one service from the repository root so it can import the shared
# protobuf packages generated from proto/*.proto.
FROM golang:1.25.4-bookworm AS builder

WORKDIR /src
RUN apt-get update \
    && apt-get install -y --no-install-recommends protobuf-compiler ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11 \
    && go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1

ENV PATH="/go/bin:${PATH}"
RUN protoc --go_out=. --go_opt=module=digital-metro/ecomm_java \
    --go-grpc_out=. --go-grpc_opt=module=digital-metro/ecomm_java \
    proto/*/*.proto \
    && go mod tidy \
    && go mod download

ARG SERVICE
RUN test -n "${SERVICE}" && test -d "/src/services/${SERVICE}"
WORKDIR /src/services/${SERVICE}
RUN go mod download \
    && mkdir -p /out \
    && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/service .

# Run as a non-root user in a minimal image.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/service /service
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/service"]
