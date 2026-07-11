# syntax=docker/dockerfile:1.4
########################################################################################################################
# Omnibus Image
########################################################################################################################

######## Build the frontend
FROM --platform=${BUILDPLATFORM} node AS frontendbuild
WORKDIR /go/src/github.com/analogj/scrutiny
COPY --link . /go/src/github.com/analogj/scrutiny

RUN make binary-frontend


######## Build the backend
FROM golang:1.26-trixie AS backendbuild

WORKDIR /go/src/github.com/analogj/scrutiny
COPY --link . /go/src/github.com/analogj/scrutiny
RUN apt-get update && DEBIAN_FRONTEND=noninteractive \
    apt-get install -y --no-install-recommends \
    file \
    && rm -rf /var/lib/apt/lists/*
RUN make binary-clean binary-all WEB_BINARY_NAME=scrutiny


######## Combine build artifacts in runtime image
FROM debian:trixie-slim AS runtime
ARG TARGETARCH
EXPOSE 8080
WORKDIR /opt/scrutiny
ENV PATH="/opt/scrutiny/bin:${PATH}"
ENV INFLUXD_CONFIG_PATH=/opt/scrutiny/influxdb
ENV S6VER="3.1.6.2"
ENV INFLUXVER="2.9.1-1"
ENV S6_SERVICES_READYTIME=1000
SHELL ["/usr/bin/sh", "-c"]

RUN sed -i 's/Components: main/Components: main contrib/' /etc/apt/sources.list.d/debian.sources \
    && echo "deb http://deb.debian.org/debian trixie-backports main" > /etc/apt/sources.list.d/backports.list \
    && apt-get update && DEBIAN_FRONTEND=noninteractive \
    apt-get install -y --no-install-recommends \
    ca-certificates \
    cron \
    curl \
    python3 \
    python3-venv \
    tzdata \
    procps \
    xz-utils \
    btrfs-progs \
    zfsutils-linux \
    mdadm \
    fio \
    && apt-get install -y --no-install-recommends -t trixie-backports smartmontools \
    && rm -rf /var/lib/apt/lists/* \
    && update-ca-certificates \
    && download_and_extract_xz() { \
           url="$1"; \
           archive="$2"; \
           checksum_file="${archive}.sha256"; \
           curl --fail --location --show-error --silent \
               --retry 5 --retry-all-errors --retry-delay 2 --max-time 300 \
               "$url" --output "$archive" \
           && curl --fail --location --show-error --silent \
               --retry 5 --retry-all-errors --retry-delay 2 --max-time 60 \
               "${url}.sha256" --output "$checksum_file" \
           && awk -v archive="$archive" '{print $1 "  " archive}' "$checksum_file" | sha256sum -c - \
           && xz -t "$archive" \
           && tar -Jxpf "$archive" -C / \
           && rm -f "$archive" "$checksum_file"; \
       } \
    && case ${TARGETARCH} in \
           "amd64")  S6_ARCH=x86_64  ;; \
           "arm64")  S6_ARCH=aarch64  ;; \
       esac \
    && mkdir -p /opt/scrutiny/bin \
    && python3 -m venv /opt/scrutiny/apprise-venv \
    && /opt/scrutiny/apprise-venv/bin/pip install --no-cache-dir apprise \
    && ln -sf /opt/scrutiny/apprise-venv/bin/apprise /opt/scrutiny/bin/apprise \
    && download_and_extract_xz \
        "https://github.com/just-containers/s6-overlay/releases/download/v${S6VER}/s6-overlay-noarch.tar.xz" \
        /tmp/s6-overlay-noarch.tar.xz \
    && download_and_extract_xz \
        "https://github.com/just-containers/s6-overlay/releases/download/v${S6VER}/s6-overlay-${S6_ARCH}.tar.xz" \
        /tmp/s6-overlay-${S6_ARCH}.tar.xz
RUN curl -L https://dl.influxdata.com/influxdb/releases/influxdb2_${INFLUXVER}_${TARGETARCH}.deb --output /tmp/influxdb2-${INFLUXVER}-${TARGETARCH}.deb \
    && dpkg -i --force-all /tmp/influxdb2-${INFLUXVER}-${TARGETARCH}.deb \
    && rm -rf /tmp/influxdb2-${INFLUXVER}-${TARGETARCH}.deb

COPY /rootfs /
COPY /docker/apparmor-profile /opt/scrutiny/apparmor-profile

COPY --link --from=backendbuild --chmod=755 /go/src/github.com/analogj/scrutiny/scrutiny /opt/scrutiny/bin/
COPY --link --from=backendbuild --chmod=755 /go/src/github.com/analogj/scrutiny/scrutiny-collector-metrics /opt/scrutiny/bin/
COPY --link --from=backendbuild --chmod=755 /go/src/github.com/analogj/scrutiny/scrutiny-collector-zfs /opt/scrutiny/bin/
COPY --link --from=backendbuild --chmod=755 /go/src/github.com/analogj/scrutiny/scrutiny-collector-performance /opt/scrutiny/bin/
COPY --link --from=backendbuild --chmod=755 /go/src/github.com/analogj/scrutiny/scrutiny-collector-mdadm /opt/scrutiny/bin/
COPY --link --from=backendbuild --chmod=755 /go/src/github.com/analogj/scrutiny/scrutiny-collector-filesystem /opt/scrutiny/bin/
COPY --link --from=backendbuild --chmod=755 /go/src/github.com/analogj/scrutiny/scrutiny-collector-btrfs /opt/scrutiny/bin/
COPY --link --from=frontendbuild --chmod=644 /go/src/github.com/analogj/scrutiny/webapp/frontend/dist/treo/browser /opt/scrutiny/web
COPY --link --from=backendbuild --chmod=644 /go/src/github.com/analogj/scrutiny/docs/openapi.yaml /opt/scrutiny/docs/openapi.yaml
COPY --link --from=backendbuild --chmod=644 /go/src/github.com/analogj/scrutiny/docs/swagger-ui.html /opt/scrutiny/docs/swagger-ui.html
RUN chmod 0644 /etc/cron.d/scrutiny && \
    chmod 0644 /etc/cron.d/scrutiny-zfs && \
    chmod 0644 /etc/cron.d/scrutiny-performance && \
    chmod 0644 /etc/cron.d/scrutiny-mdadm && \
    chmod 0644 /etc/cron.d/scrutiny-filesystem && \
    chmod 0644 /etc/cron.d/scrutiny-btrfs && \
    rm -f /etc/cron.daily/* && \
    mkdir -p /opt/scrutiny/web && \
    mkdir -p /opt/scrutiny/docs && \
    mkdir -p /opt/scrutiny/config && \
    chmod -R ugo+rwx /opt/scrutiny/config && \
    chmod +x /etc/cont-init.d/* && \
    chmod +x /etc/services.d/*/run && \
    chmod +x /etc/services.d/*/finish

ENTRYPOINT ["/init"]
