FROM registry.golang.scot/systemd:latest

ENV APP_ENV prod

COPY dockerfiles/router.service dockerfiles/router.socket /usr/lib/systemd/system/

RUN systemctl enable router.socket; \
mkdir -p /etc/router;

COPY router /usr/local/bin/

EXPOSE 80 443
CMD ["/sbin/init"]
