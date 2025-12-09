FROM golang:1.24-alpine AS builder
WORKDIR /app
RUN apk add --no-cache build-base git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/post-service ./cmd/post-service

FROM gcr.io/distroless/base-debian12:nonroot
WORKDIR /app
COPY --from=builder /out/post-service /app/service
ENTRYPOINT ["/app/service"]

