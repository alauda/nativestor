# Build the manager binaryFROM golang:1.13 as builder

FROM golang:1.15 AS builder
COPY . /workdir
WORKDIR /workdir
RUN go build -o /workdir/bin/topolvm main.go

FROM ubuntu:18.04
RUN apt-get update && apt-get -y install gdisk udev
COPY --from=builder /workdir/bin/topolvm /topolvm
ENTRYPOINT ["/topolvm"]
