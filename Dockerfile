# Stage 1: Build the binary
FROM --platform=linux/amd64 debian:trixie AS builder

# Install dependencies: Go, git, and sed
RUN apt-get update && apt-get install -y \
    golang-go \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src

# Copy the source code from the current directory
COPY . .

# Download dependencies and build the binary
RUN go mod tidy && \
    go build -ldflags="-s -w" -o haproxy-mapper .

# Stage 2: Export stage
# This stage is minimal and just holds the binary for extraction
FROM scratch AS export
COPY --from=builder /src/haproxy-mapper /haproxy-mapper
