ARG kindBase
FROM ${kindBase}
RUN apt-get update && apt-get -y install lvm2
RUN sed -i -e 's/udev_sync =.*/udev_sync = 0/' \
           -e 's/udev_rules =.*/udev_rules = 0/'  /etc/lvm/lvm.conf