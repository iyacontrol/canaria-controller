# Build the manager binary
FROM golang:1.14-alpine3.11 as builder

# Copy in the go src
WORKDIR /go/src/github.com/iyacontrol/canaria-controller
COPY pkg/    pkg/
COPY cmd/    cmd/
COPY vendor/ vendor/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build  -o manager github.com/iyacontrol/canaria-controller/cmd/manager

# Copy the controller-manager into a thin image
FROM alpine:3.11
WORKDIR /root/
COPY --from=builder /go/src/github.com/iyacontrol/canaria-controller/manager .
ENTRYPOINT ["./manager"]