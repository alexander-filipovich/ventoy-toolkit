FROM debian:stable-slim

ARG VENTOY_VERSION=1.0.95
ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
       bash \
       ca-certificates \
       curl \
       tar \
       xz-utils \
       util-linux \
       fdisk \
       parted \
       dosfstools \
       udev \
       coreutils \
    && rm -rf /var/lib/apt/lists/*

RUN curl -fsSL -o /tmp/ventoy.tar.gz \
      "https://github.com/ventoy/Ventoy/releases/download/v${VENTOY_VERSION}/ventoy-${VENTOY_VERSION}-linux.tar.gz" \
    && mkdir -p /opt \
    && tar -xzf /tmp/ventoy.tar.gz -C /opt \
    && mv "/opt/ventoy-${VENTOY_VERSION}" /opt/ventoy \
    && rm -f /tmp/ventoy.tar.gz

RUN printf '%s\n' '#!/usr/bin/env bash' \
    'set -euo pipefail' \
    'exec /opt/ventoy/Ventoy2Disk.sh "$@"' \
    > /usr/local/bin/ventoy \
    && chmod +x /usr/local/bin/ventoy

WORKDIR /workspace
ENTRYPOINT ["ventoy"]
CMD ["--help"]
