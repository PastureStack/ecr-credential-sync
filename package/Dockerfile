ARG UBUNTU_IMAGE=ubuntu:26.04@sha256:3131b4cc82a783df6c9df078f86e01819a13594b865c2cad47bd1bca2b7063bb
FROM ${UBUNTU_IMAGE}

# Modified by PastureStack contributors for independent maintenance and rebranding.

ARG UBUNTU_MIRROR=http://archive.ubuntu.com/ubuntu
ARG VERSION
ARG REVISION
ENV DEBIAN_FRONTEND=noninteractive

RUN set -eux; \
    if [ -f /etc/apt/sources.list.d/ubuntu.sources ]; then \
        sed -i "s|http://archive.ubuntu.com/ubuntu|${UBUNTU_MIRROR}|g; s|http://security.ubuntu.com/ubuntu|${UBUNTU_MIRROR}|g" /etc/apt/sources.list.d/ubuntu.sources; \
    fi; \
    printf 'Acquire::Retries "5";\nAcquire::http::Timeout "30";\nAcquire::https::Timeout "30";\n' > /etc/apt/apt.conf.d/80pasturestack-retries; \
    apt-get -o Acquire::Retries=5 -o Acquire::http::Timeout=30 update && \
    apt-get -y upgrade && \
    apt-get install -y --no-install-recommends ca-certificates && \
    rm -rf /var/lib/apt/lists/* && \
    rm -f /usr/bin/pebble && \
    groupadd --gid 10001 pasturestack && \
    useradd --uid 10001 --gid 10001 --create-home --home-dir /home/pasturestack --shell /usr/sbin/nologin pasturestack

COPY --chown=10001:10001 ecr-credential-sync /usr/local/bin/ecr-credential-sync
COPY LICENSE SECURITY.md ORIGIN.md COMPATIBILITY.md THIRD_PARTY_NOTICES.md /licenses/
COPY licenses/third-party/ /licenses/third-party/
RUN chmod 0755 /usr/local/bin/ecr-credential-sync && \
    find /licenses -type f -exec chmod 0644 {} +

LABEL org.opencontainers.image.title="ecr-credential-sync" \
      org.opencontainers.image.description="Independent PastureStack ECR credential synchronization service for legacy environments" \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.source="https://github.com/PastureStack/ecr-credential-sync" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${REVISION}"

ENV HOME=/home/pasturestack
USER 10001:10001
ENTRYPOINT ["/usr/local/bin/ecr-credential-sync"]
