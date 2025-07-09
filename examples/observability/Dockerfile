FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

# Copy the observability example files
COPY examples/observability/go.mod examples/observability/go.sum ./
COPY examples/observability/*.go ./

# Copy the parent sqs-go module (needed for local replacement)
COPY go.mod go.sum ../../
COPY consumer ../../consumer/

RUN go mod download

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o observability-example .

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

COPY --from=builder /app/observability-example .

# Expose the Prometheus metrics port
EXPOSE 9464

CMD ["./observability-example"]