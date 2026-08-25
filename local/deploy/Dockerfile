FROM node:24-bookworm-slim@sha256:e5a8dee7bc1e6a215d224a7ef8206f7e77271bc3cabd5febf2beafac0674f174 AS web

ARG SWAGGER_UI_VERSION=5.32.6

WORKDIR /src

RUN apt-get update \
    && apt-get install --no-install-recommends --yes ca-certificates tar wget \
    && rm -rf /var/lib/apt/lists/* \
    && corepack enable

COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./web/
RUN cd web && pnpm install --frozen-lockfile --ignore-scripts

COPY web ./web
COPY api ./api

RUN cd web \
    && pnpm build \
    && cp -r /src/api /src/internal/server/public_html \
    && wget --quiet \
        "https://github.com/swagger-api/swagger-ui/archive/v${SWAGGER_UI_VERSION}.tar.gz" \
        -O /tmp/swagger-ui.tar.gz \
    && tar \
        -C /src/internal/server/public_html/api \
        --exclude=index.html \
        --exclude='*.map' \
        --exclude='*-es-*' \
        --exclude='swagger-{ui,initializer}.js' \
        --strip-components=2 \
        -xf /tmp/swagger-ui.tar.gz \
        "swagger-ui-${SWAGGER_UI_VERSION}/dist"

FROM golang:1.26.3-bookworm@sha256:3bf5b04541eb4a37fe62aa1bc9c98a1dec09db9d2e79c1d2eb54e3c9d08dbca9 AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=web /src/internal/server/public_html ./internal/server/public_html

RUN CGO_ENABLED=1 \
    CGO_CPPFLAGS='-D_FORTIFY_SOURCE=2 -fstack-protector-strong' \
    CGO_LDFLAGS='-Wl,-z,relro,-z,now' \
    GOOS=linux \
    GOARCH=amd64 \
    go build \
      -buildmode=pie \
      -trimpath \
      -ldflags='-linkmode=external -s -w' \
      -o /out/authelia \
      ./cmd/authelia/

FROM authelia/base:4.39.20@sha256:ce6163188a6ed200060c04a5d77d68327fe164e248e4969e65da09c13451edc3

WORKDIR /app

ENV PATH="/app:${PATH}" \
    PUID=0 \
    PGID=0 \
    X_AUTHELIA_CONFIG=/config/configuration.yml

COPY --from=build --chmod=755 /out/authelia ./authelia
COPY --chmod=644 LICENSE ./LICENSE
COPY --chmod=755 entrypoint.sh healthcheck.sh ./
COPY --chmod=666 .healthcheck.env ./

EXPOSE 9091

ENTRYPOINT ["/app/entrypoint.sh"]
HEALTHCHECK --interval=30s --timeout=3s --start-period=1m CMD /app/healthcheck.sh
