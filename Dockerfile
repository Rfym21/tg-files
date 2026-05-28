FROM node:24-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.24 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /internal/web/dist/ ./internal/web/dist/
RUN CGO_ENABLED=0 go build -o /out/tgnas ./cmd/tgnas

FROM alpine:3.23
RUN apk add --no-cache ca-certificates tzdata && \
    adduser -D -H app

COPY --from=build /out/tgnas /usr/local/bin/tgnas
USER app
EXPOSE 9000
WORKDIR /app
CMD ["tgnas"]
