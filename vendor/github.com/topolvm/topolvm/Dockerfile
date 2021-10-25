# Build Container
FROM golang:1.16-buster AS build-env

# Get argment
ARG TOPOLVM_VERSION

COPY . /workdir
WORKDIR /workdir

RUN make build TOPOLVM_VERSION=${TOPOLVM_VERSION}

# TopoLVM container
FROM ubuntu:20.04

ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update \
    && apt-get -y install --no-install-recommends \
        file \
        xfsprogs \
    && rm -rf /var/lib/apt/lists/*

COPY --from=build-env /workdir/build/hypertopolvm /hypertopolvm

RUN ln -s hypertopolvm /lvmd \
    && ln -s hypertopolvm /topolvm-scheduler \
    && ln -s hypertopolvm /topolvm-node \
    && ln -s hypertopolvm /topolvm-controller

COPY --from=build-env /workdir/LICENSE /LICENSE

ENTRYPOINT ["/hypertopolvm"]
