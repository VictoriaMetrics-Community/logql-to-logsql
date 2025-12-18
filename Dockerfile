ARG GO_VERSION=1.25
ARG NODE_VERSION=24

FROM node:${NODE_VERSION}-bookworm as nbuilder
WORKDIR /usr/src/app
COPY . .
WORKDIR /usr/src/app/cmd/logql-to-logsql/web/ui
RUN npm install && npm run build

FROM golang:${GO_VERSION}-bookworm as builder
WORKDIR /usr/src/app
COPY go.mod go.sum ./
RUN go mod download && go mod verify
COPY . .
COPY --from=nbuilder /usr/src/app/cmd/logql-to-logsql/web/dist /usr/src/app/cmd/logql-to-logsql/web/dist
RUN make backend-build

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y ca-certificates --no-install-recommends && rm -rf /var/lib/apt/lists/*
RUN update-ca-certificates

# Create non-root user
RUN useradd -m -u 1000 -s /bin/bash appuser

COPY --from=builder /usr/src/app/logql-to-logsql /usr/local/bin/

# Switch to non-root user
USER appuser

EXPOSE 8080

CMD ["logql-to-logsql"]

