# Build stage
FROM golang:1.26-alpine AS build
RUN apk add --no-cache gcc musl-dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -ldflags='-s -w' -o /out/todoshka .

# Run stage
FROM alpine:3.20
RUN apk add --no-cache ca-certificates sqlite wget && \
    addgroup -S todoshka && adduser -S todoshka -G todoshka
WORKDIR /app
COPY --from=build /out/todoshka /app/todoshka
RUN mkdir -p /app/data && chown -R todoshka:todoshka /app
USER todoshka
EXPOSE 8080
ENV TODOSHKA_DB=/app/data/todoshka.db \
    TODOSHKA_PORT=:8080
VOLUME ["/app/data"]
HEALTHCHECK --interval=30s --timeout=3s --retries=3 \
  CMD wget -qO- http://localhost:8080/api/health || exit 1
ENTRYPOINT ["/app/todoshka"]
