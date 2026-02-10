FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /server ./cmd/server

FROM alpine:3.19
RUN apk add --no-cache ca-certificates \
    && addgroup -S teamvault && adduser -S teamvault -G teamvault
COPY --from=builder /server /server
COPY migrations /migrations
RUN chown -R teamvault:teamvault /migrations
USER teamvault
EXPOSE 8443
ENTRYPOINT ["/server"]
