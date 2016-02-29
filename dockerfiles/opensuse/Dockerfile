FROM opensuse:latest

ENV  PATH $PATH:/usr/local/go/bin

RUN mkdir -p /usr/local/go; \
zypper install --no-confirm curl tar git && zypper clean --all

COPY go/ /usr/local/go

CMD ["/sbin/init"]
