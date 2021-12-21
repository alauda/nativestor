# Build the manager binaryFROM golang:1.13 as builder

FROM golang:1.16 AS builder
ARG TOPOLVM_OPERATOR_VERSION
COPY . /workdir
WORKDIR /workdir
RUN make build TOPOLVM_OPERATOR_VERSION=${TOPOLVM_OPERATOR_VERSION}

FROM ubuntu:21.04
RUN apt-get update && apt-get -y install gdisk udev
COPY --from=builder /workdir/bin/topolvm /topolvm
ENTRYPOINT ["/topolvm"]
