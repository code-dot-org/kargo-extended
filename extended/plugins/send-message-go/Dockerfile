FROM golang:1.25.1-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags='-s -w' \
    -o /out/send-message-plugin ./cmd/send-message-plugin

FROM alpine:3.22

RUN apk add --no-cache ca-certificates

COPY --from=build /out/send-message-plugin /usr/local/bin/send-message-plugin

ENTRYPOINT ["/usr/local/bin/send-message-plugin"]
