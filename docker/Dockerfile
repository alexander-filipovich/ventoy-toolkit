FROM debian:stable-slim

ARG VENTOY_VERSION=1.1.12
ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
       bash \
       ca-certificates \
       coreutils \
       curl \
       tar \
       xz-utils \
       \
       bsdextrautils \
       util-linux \
       udev \
       \
       dosfstools \
       exfatprogs \
       fdisk \
       parted \
    && rm -rf /var/lib/apt/lists/*

COPY docker/bin/mkexfatfs /usr/local/bin/mkexfatfs

RUN curl -fsSL -o /tmp/ventoy.tar.gz \
      "https://github.com/ventoy/Ventoy/releases/download/v${VENTOY_VERSION}/ventoy-${VENTOY_VERSION}-linux.tar.gz" \
    && mkdir -p /opt \
    && tar -xzf /tmp/ventoy.tar.gz -C /opt \
    && mv "/opt/ventoy-${VENTOY_VERSION}" /opt/ventoy \
    && rm -f /tmp/ventoy.tar.gz

RUN printf '%s\n' '#!/usr/bin/env bash' \
    'set -euo pipefail' \
    'cd /opt/ventoy' \
    'exec ./Ventoy2Disk.sh "$@"' \
    > /usr/local/bin/ventoy \
    && chmod +x /usr/local/bin/ventoy

WORKDIR /opt/ventoy
ENTRYPOINT ["ventoy"]
CMD ["--help"]
