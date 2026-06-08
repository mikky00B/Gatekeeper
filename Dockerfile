FROM golang:1.26-alpine AS build

WORKDIR /src
COPY go.mod ./
COPY . .
RUN go test ./... && go build -buildvcs=false -o /out/gateway ./cmd/gateway && go build -buildvcs=false -o /out/gateway-cli ./cmd/cli

FROM alpine:3.20

RUN adduser -D -H gateway
WORKDIR /app
COPY --from=build /out/gateway /usr/local/bin/gateway
COPY --from=build /out/gateway-cli /usr/local/bin/gateway-cli
COPY config/exampleconfig.yaml /app/config/config.yaml
COPY internal/db/migrations /app/migrations
USER gateway
EXPOSE 8080

ENTRYPOINT ["gateway"]
