FROM golang:1.26.3-bookworm@sha256:3bf5b04541eb4a37fe62aa1bc9c98a1dec09db9d2e79c1d2eb54e3c9d08dbca9 AS build

WORKDIR /src
COPY diagnostic-app.go .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o /out/diagnostic diagnostic-app.go

FROM scratch
COPY --from=build /out/diagnostic /diagnostic
EXPOSE 8080
ENTRYPOINT ["/diagnostic"]
