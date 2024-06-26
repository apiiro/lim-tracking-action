# Code generated by ezactions. DO NOT EDIT.

FROM golang:1.16.2-alpine AS builder
RUN apk --no-cache add ca-certificates
WORKDIR	/builddir
COPY . .
RUN go build -o entrypoint . && mv entrypoint /entrypoint

FROM alpine:3.11
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /entrypoint /entrypoint

ENTRYPOINT ["/entrypoint"]
