FROM golang:1.22rc2-bookworm AS builder

ENV GOOS=linux
ENV GOARCH=amd64
ENV CGO_ENABLED=0

EXPOSE 8000

WORKDIR /app

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .
RUN go build \
  -o file-uploader \
  -tags timetzdata \
  -trimpath \
  main.go

FROM alpine:latest
RUN apk update && apk add --no-cache bash build-base ca-certificates pkgconf
COPY --from=builder /app/file-uploader /opt/file-uploader
ENTRYPOINT ["/opt/file-uploader"]
