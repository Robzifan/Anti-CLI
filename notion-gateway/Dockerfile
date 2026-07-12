# syntax=docker/dockerfile:1.7
FROM golang:1.26.5-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 go test ./... && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/gateway ./cmd/gateway
FROM alpine:3.22
RUN addgroup -S gateway && adduser -S -G gateway gateway
COPY --from=build /out/gateway /usr/local/bin/gateway
USER gateway
EXPOSE 8787
ENTRYPOINT ["gateway"]
