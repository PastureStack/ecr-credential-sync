ARG UBUNTU_IMAGE=ubuntu:26.04@sha256:678c6550cc43645e08669028bc177f50be4e7c5b8cca677067b1914d4afc7a03

FROM ${UBUNTU_IMAGE} AS snapshot-ca-bootstrap

ARG SOURCE_DATE_EPOCH

ADD --checksum=sha256:6077d27c6b6f8b23590cb01ff877ed8c804a67a5442cc32b5a33da10d2bd0e90 \
    https://snapshot.ubuntu.com/ubuntu/20260808T000000Z/pool/main/c/ca-certificates/ca-certificates_20260601~26.04.1_all.deb \
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
    test "$(stat -c '%Y' /snapshot-ca-certificates.crt)" = "${SOURCE_DATE_EPOCH}"

FROM ${UBUNTU_IMAGE}

# Modified by PastureStack contributors for independent maintenance and rebranding.

ARG UBUNTU_APT_SNAPSHOT=20260808T000000Z
ARG VERSION
ARG REVISION
ARG SOURCE_DATE_EPOCH
ARG UBUNTU_IMAGE

ENV DEBIAN_FRONTEND=noninteractive

COPY --from=snapshot-ca-bootstrap /snapshot-ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --chown=0:0 --chmod=0444 ubuntu-apt.lock /usr/share/pasturestack/manifests/ubuntu-apt.lock

RUN set -eux; \
    chown 0:0 /usr/share/pasturestack /usr/share/pasturestack/manifests; \
    chmod 0755 /usr/share/pasturestack /usr/share/pasturestack/manifests; \
    case "${SOURCE_DATE_EPOCH}" in ''|*[!0-9]*) exit 64 ;; esac; \
    test "${SOURCE_DATE_EPOCH}" -le "$(date -u +%s)"; \
    . /usr/share/pasturestack/manifests/ubuntu-apt.lock; \
    test "${UBUNTU_APT_SNAPSHOT}" = "${UBUNTU_APT_LOCKED_SNAPSHOT}"; \
    rm -f /etc/apt/sources.list /etc/apt/sources.list.d/*.list /etc/apt/sources.list.d/*.sources; \
    printf 'Types: deb\nURIs: https://snapshot.ubuntu.com/ubuntu/%s\nSuites: resolute resolute-updates resolute-backports resolute-security\nComponents: main universe restricted multiverse\nSigned-By: /usr/share/keyrings/ubuntu-archive-keyring.gpg\nSnapshot: no\n' \
        "${UBUNTU_APT_SNAPSHOT}" > /etc/apt/sources.list.d/pasturestack-snapshot.sources; \
    printf 'Acquire::Retries "5";\nAcquire::http::Timeout "30";\nAcquire::https::Timeout "30";\nAcquire::http::Pipeline-Depth "0";\nAcquire::https::CaInfo "/etc/ssl/certs/ca-certificates.crt";\nAcquire::https::Verify-Peer "true";\nAcquire::https::Verify-Host "true";\nAcquire::AllowInsecureRepositories "false";\nAPT::Get::AllowUnauthenticated "false";\n' \
        > /etc/apt/apt.conf.d/80pasturestack-snapshot; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
        ca-certificates="${UBUNTU_APT_CA_CERTIFICATES_VERSION}"; \
    test "$(dpkg-query -W -f='${Version}' ca-certificates)" = "${UBUNTU_APT_CA_CERTIFICATES_VERSION}"; \
    { \
        printf 'record\tname\tarchitecture\tversion\n'; \
        printf 'metadata\tschema\t-\tpasturestack.dpkg-manifest/v1\n'; \
        printf 'metadata\tubuntu_snapshot\t-\t%s\n' "${UBUNTU_APT_SNAPSHOT}"; \
        printf 'metadata\tbase_image\t-\t%s\n' "${UBUNTU_IMAGE}"; \
        dpkg-query -W -f='package\t${binary:Package}\t${Architecture}\t${Version}\n' | LC_ALL=C sort; \
    } > /usr/share/pasturestack/manifests/runtime-ubuntu-packages.tsv; \
    chmod 0444 \
        /usr/share/pasturestack/manifests/ubuntu-apt.lock \
        /usr/share/pasturestack/manifests/runtime-ubuntu-packages.tsv; \
    for manifest_directory in \
        /usr/share/pasturestack \
        /usr/share/pasturestack/manifests; do \
      test "$(stat -c '%u:%g:%a' "${manifest_directory}")" = '0:0:755'; \
    done; \
    for locked_manifest in \
        /usr/share/pasturestack/manifests/ubuntu-apt.lock \
        /usr/share/pasturestack/manifests/runtime-ubuntu-packages.tsv; do \
      test "$(stat -c '%u:%g:%a' "${locked_manifest}")" = '0:0:444'; \
    done; \
    apt-get clean; \
    rm -rf /var/lib/apt/lists/* /var/cache/apt/archives/* /tmp/* /var/tmp/*; \
    rm -f /var/log/apt/* /var/log/dpkg.log /var/log/alternatives.log /var/cache/ldconfig/aux-cache; \
    rm -f /usr/bin/pebble; \
    groupadd --gid 10001 pasturestack; \
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
      org.opencontainers.image.revision="${REVISION}" \
      org.opencontainers.image.base.name="${UBUNTU_IMAGE}"

ENV HOME=/home/pasturestack
USER 10001:10001
ENTRYPOINT ["/usr/local/bin/ecr-credential-sync"]
