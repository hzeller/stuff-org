ARG ARCH="amd64"
ARG OS="linux"
FROM quay.io/prometheus/busybox-${OS}-${ARCH}:latest

ADD stuff /bin/stuff

EXPOSE     9199
USER       nobody
ENTRYPOINT ["/bin/stuff"]
