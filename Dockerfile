FROM golang:1.22 AS builder
WORKDIR /app
COPY . .
RUN go mod download

# Build arguments para construir diferentes binarios
ARG SERVICE_NAME
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/app ./cmd/${SERVICE_NAME}

FROM alpine:latest
COPY --from=builder /bin/app /app
ENTRYPOINT ["/app"]