# Builds haproxy-mapper and runs geoip_generator.py: one HAProxy map per country from the
# MaxMind GeoLite2 Country database. See the README's "Per-country maps".
# Usage:
#   podman build -f geoip_generator.Containerfile -t geoip_generator .

FROM docker.io/library/golang:1-alpine AS mapper
WORKDIR /src
COPY . .
RUN go mod tidy && \
    CGO_ENABLED=0 go build -ldflags="-s -w" -o /haproxy-mapper .

FROM docker.io/library/python:3-alpine
COPY --from=mapper /haproxy-mapper /usr/local/bin/haproxy-mapper
COPY geoip_generator.py /usr/local/bin/geoip_generator.py
ENTRYPOINT ["python3", "/usr/local/bin/geoip_generator.py"]
