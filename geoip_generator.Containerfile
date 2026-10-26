# haproxy-mapper in a container, defaulting to one HAProxy map per country from the
# free MaxMind GeoLite2 Country database. See the README's "Per-country maps".
# Usage:
#   podman build -f geoip_generator.Containerfile -t geoip_generator .

FROM docker.io/library/golang:1-alpine@sha256:b8c9b82affbbb82a020bbcf998dad2ba9516944ac7e883524642a481a9505e9c AS mapper
WORKDIR /src
COPY . .
RUN go mod download && \
    CGO_ENABLED=0 go build -ldflags="-s -w" -o /haproxy-mapper .

# Root, so it can replace maps in a host directory owned by root.
FROM gcr.io/distroless/static-debian12@sha256:d75cdd72874d4790092fcb1b058493ecf6bb5bf2b2b897045b00ff01d91843f2
COPY --from=mapper /haproxy-mapper /haproxy-mapper
ENTRYPOINT ["/haproxy-mapper"]
CMD ["-geolite2", "-country-dir", "/etc/haproxy/countries"]
