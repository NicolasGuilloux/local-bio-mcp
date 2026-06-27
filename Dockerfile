# syntax=docker/dockerfile:1

# ---- build stage ----
FROM golang:1.26-alpine AS build
WORKDIR /src

# Cache modules first.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG VERSION=docker
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X main.version=${VERSION}" \
    -o /out/localbio ./cmd/localbio

# ---- runtime stage ----
FROM gcr.io/distroless/static-debian12:nonroot

# The MCP HTTP server stores its session under this dir.
ENV LOCALBIO_CONFIG_DIR=/data
ENV LOCALBIO_API_BASE=https://www.local.bio/api-v2
WORKDIR /data

COPY --from=build /out/localbio /usr/local/bin/localbio

EXPOSE 8080
USER nonroot:nonroot

ENTRYPOINT ["localbio"]
# Default: Streamable HTTP MCP server on :8080
CMD ["mcp", "http", ":8080"]
