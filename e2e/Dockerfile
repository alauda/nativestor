FROM ubuntu:20.04
RUN apt-get update && apt-get -y install gdisk udev lvm2
COPY . /
ENTRYPOINT ["/topolvm"]