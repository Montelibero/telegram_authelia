ARG BUILD_COMMIT=unknown
ARG BUILD_BRANCH=deploy
ARG BUILD_TAG=latest
ARG BUILD_STATE="untagged clean"

FROM node:24-bookworm-slim@sha256:e5a8dee7bc1e6a215d224a7ef8206f7e77271bc3cabd5febf2beafac0674f174 AS web

ARG PNPM_VERSION=11.24.0
ARG SWAGGER_UI_COMMIT=dcdca62c8b64a0a54e4decd4e1a6c3c712fdcc60
ARG SWAGGER_UI_SHA256=27741bad0e2843f615bde86629e86b48d2f7eee6cc07785defa6c025ead25656

WORKDIR /src

RUN apt-get update \
    && apt-get install --no-install-recommends --yes ca-certificates tar wget \
    && rm -rf /var/lib/apt/lists/* \
    && corepack enable \
    && corepack install --global "pnpm@${PNPM_VERSION}"

COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./web/
RUN cd web && pnpm install --frozen-lockfile --ignore-scripts

COPY web ./web
COPY api ./api

RUN cd web \
    && pnpm build \
    && cp -r /src/api /src/internal/server/public_html \
    && wget --quiet \
        "https://github.com/swagger-api/swagger-ui/archive/${SWAGGER_UI_COMMIT}.tar.gz" \
        -O /tmp/swagger-ui.tar.gz \
    && echo "${SWAGGER_UI_SHA256}  /tmp/swagger-ui.tar.gz" | sha256sum --check --strict \
    && tar \
        -C /src/internal/server/public_html/api \
        --exclude=index.html \
        --exclude='*.map' \
        --exclude='*-es-*' \
        --exclude='swagger-{ui,initializer}.js' \
        --strip-components=2 \
        -xf /tmp/swagger-ui.tar.gz \
        "swagger-ui-${SWAGGER_UI_COMMIT}/dist"

FROM golang:1.26.3-bookworm@sha256:3bf5b04541eb4a37fe62aa1bc9c98a1dec09db9d2e79c1d2eb54e3c9d08dbca9 AS build

ARG BUILD_COMMIT
ARG BUILD_BRANCH
ARG BUILD_TAG
ARG BUILD_STATE

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
      -ldflags="-linkmode=external -s -w -X 'github.com/authelia/authelia/v4/internal/utils.BuildCommit=${BUILD_COMMIT}' -X 'github.com/authelia/authelia/v4/internal/utils.BuildBranch=${BUILD_BRANCH}' -X 'github.com/authelia/authelia/v4/internal/utils.BuildTag=${BUILD_TAG}' -X 'github.com/authelia/authelia/v4/internal/utils.BuildState=${BUILD_STATE}'" \
      -o /out/authelia \
      ./cmd/authelia/

FROM authelia/base:4.39.20@sha256:ce6163188a6ed200060c04a5d77d68327fe164e248e4969e65da09c13451edc3

ARG BUILD_COMMIT
ARG BUILD_TAG

LABEL org.opencontainers.image.source="https://github.com/montelibero/authelia" \
      org.opencontainers.image.revision="${BUILD_COMMIT}" \
      org.opencontainers.image.version="${BUILD_TAG}"

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
