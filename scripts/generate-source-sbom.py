#!/usr/bin/env python3
"""Generate a deterministic CycloneDX inventory from source and toolchain locks."""

from __future__ import annotations

import argparse
import hashlib
import json
from pathlib import Path
import uuid


HOST_TOOLCHAIN_KEYS = (
    "HOST_BUILDX_VERSION",
    "HOST_BUILDX_COMMIT",
    "HOST_BUILDX_ASSET_PLATFORM",
    "HOST_BUILDX_ASSET_FILENAME",
    "HOST_BUILDX_ASSET_URL",
    "HOST_BUILDX_ASSET_SHA256",
    "HOST_BUILDX_CHECKSUMS_FILENAME",
    "HOST_BUILDX_CHECKSUMS_URL",
    "HOST_BUILDX_CHECKSUMS_SHA256",
    "HOST_BUILDKIT_VERSION",
    "HOST_BUILDKIT_IMAGE",
    "HOST_BUILDX_INSTALL_METHOD",
    "HOST_BUILDX_INSTALLER_PATH",
    "HOST_BUILDX_PLUGIN_RELATIVE_PATH",
)

UBUNTU_APT_KEYS = (
    "UBUNTU_APT_LOCKED_SNAPSHOT",
    "UBUNTU_APT_BASH_VERSION",
    "UBUNTU_APT_CA_CERTIFICATES_VERSION",
    "UBUNTU_APT_CURL_VERSION",
    "UBUNTU_APT_GCC_VERSION",
    "UBUNTU_APT_GIT_VERSION",
    "UBUNTU_APT_JQ_VERSION",
    "UBUNTU_APT_LIBC6_DEV_VERSION",
    "UBUNTU_APT_MAKE_VERSION",
    "UBUNTU_APT_TAR_VERSION",
    "UBUNTU_APT_XZ_UTILS_VERSION",
)


def parse_vendor_lock(path: Path) -> list[tuple[str, str]]:
    components: list[tuple[str, str]] = []
    for raw_line in path.read_text(encoding="utf-8").splitlines():
        line = raw_line.strip()
        if not line or line.startswith("#"):
            continue
        fields = line.split()
        if len(fields) == 1:
            continue
        if len(fields) != 2:
            raise ValueError(f"invalid vendor lock entry: {raw_line!r}")
        components.append((fields[0], fields[1]))
    if not components:
        raise ValueError("vendor lock contains no versioned dependencies")
    return components


def parse_host_toolchain_lock(path: Path) -> tuple[dict[str, str], str]:
    lock_text = path.read_text(encoding="utf-8")
    canonical_text = lock_text.replace("\r\n", "\n").replace("\r", "\n")
    values: dict[str, str] = {}
    for raw_line in canonical_text.splitlines():
        line = raw_line.strip()
        if not line or line.startswith("#"):
            continue
        key, separator, value = line.partition("=")
        if not separator or key not in HOST_TOOLCHAIN_KEYS or not value:
            raise ValueError(f"invalid host toolchain lock entry: {raw_line!r}")
        if key in values:
            raise ValueError(f"duplicate host toolchain lock entry: {key}")
        values[key] = value
    missing = sorted(set(HOST_TOOLCHAIN_KEYS) - values.keys())
    if missing:
        raise ValueError(f"host toolchain lock is missing keys: {', '.join(missing)}")
    lock_digest = hashlib.sha256(canonical_text.encode("utf-8")).hexdigest()
    return values, lock_digest


def parse_ubuntu_apt_lock(path: Path) -> tuple[dict[str, str], str]:
    lock_text = path.read_text(encoding="utf-8")
    canonical_text = lock_text.replace("\r\n", "\n").replace("\r", "\n")
    values: dict[str, str] = {}
    for raw_line in canonical_text.splitlines():
        line = raw_line.strip()
        if not line or line.startswith("#"):
            continue
        key, separator, quoted_value = line.partition("=")
        if (
            not separator
            or key not in UBUNTU_APT_KEYS
            or len(quoted_value) < 2
            or not quoted_value.startswith("'")
            or not quoted_value.endswith("'")
            or "'" in quoted_value[1:-1]
        ):
            raise ValueError(f"invalid Ubuntu APT lock entry: {raw_line!r}")
        if key in values:
            raise ValueError(f"duplicate Ubuntu APT lock entry: {key}")
        values[key] = quoted_value[1:-1]
    missing = sorted(set(UBUNTU_APT_KEYS) - values.keys())
    if missing:
        raise ValueError(f"Ubuntu APT lock is missing keys: {', '.join(missing)}")
    lock_digest = hashlib.sha256(canonical_text.encode("utf-8")).hexdigest()
    return values, lock_digest


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--lock", type=Path, default=Path("vendor.conf"))
    parser.add_argument(
        "--host-toolchain-lock",
        type=Path,
        default=Path("toolchain/host-build-toolchain.lock"),
    )
    parser.add_argument(
        "--ubuntu-apt-lock",
        type=Path,
        default=Path("ubuntu-apt.lock"),
    )
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()

    lock_text = args.lock.read_text(encoding="utf-8")
    lock_bytes = lock_text.replace("\r\n", "\n").replace("\r", "\n").encode("utf-8")
    dependencies = parse_vendor_lock(args.lock)
    lock_digest = hashlib.sha256(lock_bytes).hexdigest()
    host_toolchain, host_toolchain_digest = parse_host_toolchain_lock(
        args.host_toolchain_lock
    )
    ubuntu_apt, ubuntu_apt_digest = parse_ubuntu_apt_lock(args.ubuntu_apt_lock)
    serial = uuid.uuid5(
        uuid.NAMESPACE_URL,
        "pasturestack:ecr-credential-sync:"
        f"{lock_digest}:{host_toolchain_digest}:{ubuntu_apt_digest}",
    )

    components = []
    for name, version in dependencies:
        purl = f"pkg:golang/{name}@{version}"
        components.append(
            {
                "type": "library",
                "bom-ref": purl,
                "group": name.rpartition("/")[0],
                "name": name.rpartition("/")[2],
                "version": version,
                "purl": purl,
                "properties": [
                    {"name": "pasturestack:source-lock", "value": "vendor.conf"}
                ],
            }
        )

    document = {
        "bomFormat": "CycloneDX",
        "specVersion": "1.6",
        "serialNumber": f"urn:uuid:{serial}",
        "version": 1,
        "metadata": {
            "component": {
                "type": "application",
                "bom-ref": "pkg:github/PastureStack/ecr-credential-sync@3.1.0",
                "group": "PastureStack",
                "name": "ecr-credential-sync",
                "version": "3.1.0",
                "purl": "pkg:github/PastureStack/ecr-credential-sync@3.1.0",
            },
            "properties": [
                {"name": "pasturestack:vendor-lock-sha256", "value": lock_digest},
                {
                    "name": "pasturestack:host-build-toolchain-lock-sha256",
                    "value": host_toolchain_digest,
                },
                {
                    "name": "pasturestack:ubuntu-apt-lock-sha256",
                    "value": ubuntu_apt_digest,
                },
                {
                    "name": "pasturestack:ubuntu-apt-snapshot",
                    "value": ubuntu_apt["UBUNTU_APT_LOCKED_SNAPSHOT"],
                },
                {
                    "name": "pasturestack:dapper-jq-version",
                    "value": ubuntu_apt["UBUNTU_APT_JQ_VERSION"],
                },
                {
                    "name": "pasturestack:runtime-image-export",
                    "value": "docker-archive-rewrite-timestamp+explicit-image-load",
                },
                {
                    "name": "pasturestack:host-buildx-version",
                    "value": host_toolchain["HOST_BUILDX_VERSION"],
                },
                {
                    "name": "pasturestack:host-buildx-commit",
                    "value": host_toolchain["HOST_BUILDX_COMMIT"],
                },
                {
                    "name": "pasturestack:host-buildx-asset-platform",
                    "value": host_toolchain["HOST_BUILDX_ASSET_PLATFORM"],
                },
                {
                    "name": "pasturestack:host-buildx-asset-filename",
                    "value": host_toolchain["HOST_BUILDX_ASSET_FILENAME"],
                },
                {
                    "name": "pasturestack:host-buildx-asset-url",
                    "value": host_toolchain["HOST_BUILDX_ASSET_URL"],
                },
                {
                    "name": "pasturestack:host-buildx-asset-sha256",
                    "value": host_toolchain["HOST_BUILDX_ASSET_SHA256"],
                },
                {
                    "name": "pasturestack:host-buildx-checksums-filename",
                    "value": host_toolchain["HOST_BUILDX_CHECKSUMS_FILENAME"],
                },
                {
                    "name": "pasturestack:host-buildx-checksums-url",
                    "value": host_toolchain["HOST_BUILDX_CHECKSUMS_URL"],
                },
                {
                    "name": "pasturestack:host-buildx-checksums-sha256",
                    "value": host_toolchain["HOST_BUILDX_CHECKSUMS_SHA256"],
                },
                {
                    "name": "pasturestack:host-buildkit-version",
                    "value": host_toolchain["HOST_BUILDKIT_VERSION"],
                },
                {
                    "name": "pasturestack:host-buildkit-image",
                    "value": host_toolchain["HOST_BUILDKIT_IMAGE"],
                },
                {
                    "name": "pasturestack:host-buildx-install-method",
                    "value": host_toolchain["HOST_BUILDX_INSTALL_METHOD"],
                },
                {
                    "name": "pasturestack:host-buildx-installer-path",
                    "value": host_toolchain["HOST_BUILDX_INSTALLER_PATH"],
                },
                {
                    "name": "pasturestack:host-buildx-plugin-relative-path",
                    "value": host_toolchain["HOST_BUILDX_PLUGIN_RELATIVE_PATH"],
                },
            ],
        },
        "components": components,
    }

    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(
        json.dumps(document, ensure_ascii=True, indent=2, sort_keys=True) + "\n",
        encoding="utf-8",
        newline="\n",
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
