FROM golang:1.16 AS build-env

COPY . /workdir
WORKDIR /workdir

RUN make build/raw-device

# TopoLVM container
FROM ubuntu:21.10

COPY --from=build-env /workdir/build/raw-device /raw-device

RUN chmod +x /raw-device

RUN ln -s raw-device /raw-device-plugin  && ln -s raw-device /raw-device-provisioner

ENTRYPOINT ["/raw-device"]