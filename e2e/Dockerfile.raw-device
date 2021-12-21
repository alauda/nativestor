FROM ubuntu:21.10

COPY . /

RUN chmod +x /raw-device

RUN ln -s raw-device /raw-device-plugin  && ln -s raw-device /raw-device-provisioner

ENTRYPOINT ["/raw-device"]