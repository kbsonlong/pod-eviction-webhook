# Build stage
FROM --platform=$BUILDPLATFORM golang:1.20-alpine AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
ARG TARGETPLATFORM
ARG BUILDPLATFORM
RUN GOOS=$(echo $TARGETPLATFORM | cut -d/ -f1) \
    GOARCH=$(echo $TARGETPLATFORM | cut -d/ -f2) \
    CGO_ENABLED=0 go build -o webhook cmd/webhook/main.go

# Final stage
FROM --platform=$TARGETPLATFORM alpine:3.18

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/webhook .

# Create directory for certificates
RUN mkdir -p /tmp/k8s-webhook-server/serving-certs

# Run the application
CMD ["./webhook"] 