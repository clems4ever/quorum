# syntax=docker/dockerfile:1

# ---- Web client ----
FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY web/ ./
RUN npm run build

# ---- Go binaries ----
FROM golang:1.26-alpine AS go
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/quorum ./cmd/quorum \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/fakegoogle ./cmd/fakegoogle \
 && mkdir /out/data

# ---- Demo image: app + fake Google sign-in on one port (9090) ----
FROM alpine:3.22 AS demo
RUN adduser -D -H -u 10001 quorum && mkdir /data && chown quorum /data
WORKDIR /app
COPY --from=go /out/quorum /out/fakegoogle /app/bin/
COPY --from=web /src/web/dist /app/web
COPY scripts/demo.sh /app/demo.sh
ENV STATIC_DIR=/app/web DATABASE_PATH=/data/quorum.db BIN=/app/bin PORT=9090
USER quorum
VOLUME /data
EXPOSE 9090
ENTRYPOINT ["/app/demo.sh"]

# ---- Production image (default target) ----
FROM gcr.io/distroless/static-debian12:nonroot AS prod
WORKDIR /app
COPY --from=go /out/quorum /app/quorum
COPY --from=web /src/web/dist /app/web
# distroless has no shell: ship an empty data dir owned by the nonroot user.
COPY --from=go --chown=65532:65532 /out/data /data
ENV ADDR=:8080 STATIC_DIR=/app/web DATABASE_PATH=/data/quorum.db
VOLUME /data
EXPOSE 8080
ENTRYPOINT ["/app/quorum"]
CMD ["serve"]
