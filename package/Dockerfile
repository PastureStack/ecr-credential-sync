ARG UBUNTU_IMAGE=ubuntu:26.04@sha256:2260313b31c8c011cd2eebe728008efac1b3982be73eb71348ea2648d2c0e09b

FROM ${UBUNTU_IMAGE} AS snapshot-ca-bootstrap

ARG SOURCE_DATE_EPOCH

ADD --checksum=sha256:6077d27c6b6f8b23590cb01ff877ed8c804a67a5442cc32b5a33da10d2bd0e90 \
    https://snapshot.ubuntu.com/ubuntu/20260825T000000Z/pool/main/c/ca-certificates/ca-certificates_20260601~26.04.1_all.deb \
    /tmp/ca-certificates.deb

RUN set -eux; \
    case "${SOURCE_DATE_EPOCH}" in ''|*[!0-9]*) exit 64 ;; esac; \
    test "${SOURCE_DATE_EPOCH}" -le "$(date -u +%s)"; \
    rm -rf /tmp/ca-bootstrap; \
    mkdir -p /tmp/ca-bootstrap; \
    dpkg-deb --extract /tmp/ca-certificates.deb /tmp/ca-bootstrap; \
    find /tmp/ca-bootstrap/usr/share/ca-certificates -type f -name '*.crt' \
        | LC_ALL=C sort \
        | while IFS= read -r certificate; do sed -e '$a\' "${certificate}"; done \
        > /snapshot-ca-certificates.crt; \
    test -s /snapshot-ca-certificates.crt; \
    touch --no-dereference --date="@${SOURCE_DATE_EPOCH}" /snapshot-ca-certificates.crt; \
    test "$(stat -c '%Y' /snapshot-ca-certificates.crt)" = "${SOURCE_DATE_EPOCH}"; \
    mkdir -p /runtime-home; \
    chown 10001:10001 /runtime-home; \
    chmod 0700 /runtime-home

FROM scratch

# Modified by PastureStack contributors for independent maintenance and rebranding.

ARG VERSION
ARG REVISION

COPY --from=snapshot-ca-bootstrap --chown=0:0 --chmod=0444 /snapshot-ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=snapshot-ca-bootstrap --chown=10001:10001 --chmod=0700 /runtime-home/ /home/pasturestack/
COPY --chown=10001:10001 --chmod=0555 ecr-credential-sync /usr/local/bin/ecr-credential-sync
COPY --chown=0:0 LICENSE SECURITY.md ORIGIN.md COMPATIBILITY.md THIRD_PARTY_NOTICES.md /licenses/
COPY --chown=0:0 licenses/third-party/ /licenses/third-party/

LABEL org.opencontainers.image.title="ecr-credential-sync" \
      org.opencontainers.image.description="Independent PastureStack ECR credential synchronization service for legacy environments" \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.source="https://github.com/PastureStack/ecr-credential-sync" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${REVISION}" \
      org.opencontainers.image.base.name="scratch"

ENV HOME=/home/pasturestack \
    SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
USER 10001:10001
ENTRYPOINT ["/usr/local/bin/ecr-credential-sync"]
