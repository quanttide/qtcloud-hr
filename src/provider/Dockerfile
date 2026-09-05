FROM rust:1-alpine AS qtrecurit-build

WORKDIR /src/qtrecurit/src/cli

RUN apk add --no-cache musl-dev

COPY third_party/qtrecurit/src/cli/Cargo.toml third_party/qtrecurit/src/cli/Cargo.lock ./
COPY third_party/qtrecurit/src/cli/src ./src
COPY third_party/qtrecurit/src/cli/templates ./templates
RUN cargo build --release --locked --bin qtrecurit

FROM golang:1.26-alpine AS build

WORKDIR /src

ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY}

COPY src/provider/go.mod* ./
RUN go mod download

COPY src/provider/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/human-server ./cmd/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates curl nodejs npm \
    && npm install -g @larksuite/cli@1.0.79 \
    && npm cache clean --force \
    && addgroup -S app \
    && adduser -S -G app app
COPY --from=build /out/human-server /usr/local/bin/human-server
COPY --from=qtrecurit-build /src/qtrecurit/src/cli/target/release/qtrecurit /usr/local/bin/qtrecurit

RUN test -x /usr/local/bin/human-server \
    && qtrecurit --version \
    && lark-cli --version \
    && curl --version >/dev/null

USER app
EXPOSE 8080

ENV QTRECURIT_BIN=/usr/local/bin/qtrecurit

ENTRYPOINT ["human-server"]
CMD []
