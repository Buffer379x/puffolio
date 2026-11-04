# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM node:24.22.2-alpine3.24@sha256:8a32c8cf649a49915a66e81e7a609223c708d0adabf288ee3f4077d765542f85 AS frontend
WORKDIR /src/web
RUN npm install --global pnpm@12.16.1
COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml web/.npmrc ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm build

FROM --platform=$BUILDPLATFORM golang:1.27.3-alpine3.24@sha256:d200d2dd76b03a99b52651fd1325aff7dcd35422c35c01bac56571b0507f3deb AS backend
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
COPY web/embed.go web/embed.go
COPY --from=frontend /src/web/dist/ web/dist/
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/puffolio ./cmd/puffolio

FROM gcr.io/distroless/static-debian13:nonroot@sha256:25ac8f0c3b2334a9eb05452266bd4697864a8c8ef0ef71e5e6480d7f92868329
ARG VERSION=dev
LABEL org.opencontainers.image.title="Puffolio" \
      org.opencontainers.image.description="Selbst gehostete Liquidverwaltung" \
      org.opencontainers.image.source="https://github.com/Buffer379x/puffolio" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.licenses="MIT"
ENV PUFFOLIO_DATA_DIR=/data \
    PUFFOLIO_HTTP_ADDR=:8080 \
    PUFFOLIO_COOKIE_SECURE=false \
    TZ=Europe/Berlin
COPY --from=backend /out/puffolio /puffolio
VOLUME ["/data"]
EXPOSE 8080
USER 65532:65532
HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 CMD ["/puffolio", "healthcheck"]
STOPSIGNAL SIGTERM
ENTRYPOINT ["/puffolio"]
