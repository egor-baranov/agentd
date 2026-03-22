FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN mkdir -p /out \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/agentd-api ./cmd/agentd-api \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/agentd-worker ./cmd/agentd-worker \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/agentd-node ./cmd/agentd-node \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/acp-supervisor ./cmd/acp-supervisor

FROM alpine:3.21

RUN apk add --no-cache ca-certificates

WORKDIR /app

RUN mkdir -p /app/examples /var/lib/agentd/node /var/lib/agentd/artifacts

COPY --from=build /out/agentd-api /app/agentd-api
COPY --from=build /out/agentd-worker /app/agentd-worker
COPY --from=build /out/agentd-node /app/agentd-node
COPY --from=build /out/acp-supervisor /app/acp-supervisor
COPY examples/registry.json /app/examples/registry.json

EXPOSE 8080 9091

ENTRYPOINT ["/app/agentd-api"]

