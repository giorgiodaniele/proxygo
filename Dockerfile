#
# Use the official Go image to create a binary
#
FROM golang:1.24-bookworm AS builder

WORKDIR /app
COPY go.* ./
RUN  go mod download
COPY . ./
RUN go build -v -o server

#
# Use the official Debian slim image for a lean production container.
#
FROM debian:bookworm-slim
RUN set -x && apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=builder /app/server /app/server
CMD ["/app/server"]