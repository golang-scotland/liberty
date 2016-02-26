FROM registry.golang.scot/systemd:latest

ENV APP_ENV prod

COPY dockerfiles/liberty.service dockerfiles/liberty.socket /usr/lib/systemd/system/

RUN systemctl enable liberty.socket; \
mkdir -p /etc/router;

COPY liberty /usr/local/bin/

EXPOSE 80 443
CMD ["/sbin/init"]
