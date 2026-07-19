FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -o /rss-to-bsky ./cmd/rss-to-bsky

FROM alpine:3.20

RUN apk add --no-cache ca-certificates

COPY --from=build /rss-to-bsky /rss-to-bsky

VOLUME ["/data"]

ENTRYPOINT ["/rss-to-bsky"]
