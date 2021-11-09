# Build the manager binaryFROM golang:1.13 as builder

FROM golang:1.16 AS builder
ARG TOPOLVM_OPERATOR_VERSION
COPY . /workdir
WORKDIR /workdir

RUN GOPROXY=https://goproxy.cn make build TOPOLVM_OPERATOR_VERSION=${TOPOLVM_OPERATOR_VERSION}

FROM ubuntu:18.04
RUN apt-get update && apt-get -y install gdisk udev
COPY --from=builder /workdir/build/topolvm /topolvm
ENTRYPOINT ["/topolvm"]
