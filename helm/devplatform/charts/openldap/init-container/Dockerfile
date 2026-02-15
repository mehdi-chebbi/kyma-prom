FROM golang:1.21-alpine AS builder
RUN apk add --no-cache git ca-certificates
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY main.go .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /build/ldap-init .

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /build/ldap-init /ldap-init
ENTRYPOINT ["/ldap-init"]
