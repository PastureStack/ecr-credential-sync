#!/usr/bin/env python3
"""Generate a deterministic CycloneDX inventory from source and toolchain locks."""

from __future__ import annotations

import argparse
import hashlib
import json
from pathlib import Path
import uuid


DAPPER_GO_VERSION = "1.27.0"
DAPPER_GO_TELEMETRY_MODE = "off"
DAPPER_GO_TELEMETRY_DIRECTORY = "/tmp/go-config/go/telemetry"
DAPPER_GO_TELEMETRY_MODE_DATE_SOURCE = "SOURCE_DATE_EPOCH-UTC"
DAPPER_GO_TELEMETRY_MODE_FORMAT = "off YYYY-MM-DD"


HOST_TOOLCHAIN_KEYS = (
    "HOST_BUILDX_VERSION",
    "HOST_BUILDX_COMMIT",
    "HOST_BUILDX_SOURCE_DATE_EPOCH",
    "HOST_BUILDX_SOURCE_FILENAME",
    "HOST_BUILDX_SOURCE_URL",
    "HOST_BUILDX_SOURCE_SHA256",
    "HOST_BUILDX_SECURITY_PATCH_PATH",
    "HOST_BUILDX_SECURITY_PATCH_SHA256",
    "HOST_BUILDX_GO_VERSION",
    "HOST_BUILDX_GO_PLATFORM",
    "HOST_BUILDX_GO_FILENAME",
    "HOST_BUILDX_GO_URL",
    "HOST_BUILDX_GO_SHA256",
    "HOST_BUILDX_GO_ARCHIVE_VERSION",
    "HOST_BUILDX_X_MOD_VERSION",
    "HOST_BUILDX_BINARY_SHA256",
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


def parse_vendor_modules(path: Path) -> list[tuple[str, str]]:
    components: list[tuple[str, str]] = []
    for raw_line in path.read_text(encoding="utf-8").splitlines():
        line = raw_line.strip()
        if not line.startswith("# ") or line.startswith("## "):
            continue
        fields = line[2:].split()
        if "=>" in fields:
            raise ValueError(f"replaced vendor module is not supported: {raw_line!r}")
        if len(fields) < 2:
            continue
        components.append((fields[0], fields[1]))
    if not components:
        raise ValueError("vendor module manifest contains no versioned dependencies")
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
    parser.add_argument("--lock", type=Path, default=Path("vendor/modules.txt"))
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
    dependencies = parse_vendor_modules(args.lock)
    lock_digest = hashlib.sha256(lock_bytes).hexdigest()
    host_toolchain, host_toolchain_digest = parse_host_toolchain_lock(
        args.host_toolchain_lock
    )
    ubuntu_apt, ubuntu_apt_digest = parse_ubuntu_apt_lock(args.ubuntu_apt_lock)
    serial = uuid.uuid5(
        uuid.NAMESPACE_URL,
        "pasturestack:ecr-credential-sync:"
        f"{lock_digest}:{host_toolchain_digest}:{ubuntu_apt_digest}:"
        f"{DAPPER_GO_VERSION}:{DAPPER_GO_TELEMETRY_MODE}:"
        f"{DAPPER_GO_TELEMETRY_DIRECTORY}:"
        f"{DAPPER_GO_TELEMETRY_MODE_DATE_SOURCE}:"
        f"{DAPPER_GO_TELEMETRY_MODE_FORMAT}",
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
                    {
                        "name": "pasturestack:source-lock",
                        "value": "vendor/modules.txt",
                    }
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
                {
                    "name": "pasturestack:go-vendor-modules-sha256",
                    "value": lock_digest,
                },
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
                    "name": "pasturestack:dapper-go-version",
                    "value": DAPPER_GO_VERSION,
                },
                {
                    "name": "pasturestack:dapper-go-telemetry-mode",
                    "value": DAPPER_GO_TELEMETRY_MODE,
                },
                {
                    "name": "pasturestack:dapper-go-telemetry-directory",
                    "value": DAPPER_GO_TELEMETRY_DIRECTORY,
                },
                {
                    "name": "pasturestack:dapper-go-telemetry-mode-date-source",
                    "value": DAPPER_GO_TELEMETRY_MODE_DATE_SOURCE,
                },
                {
                    "name": "pasturestack:dapper-go-telemetry-mode-format",
                    "value": DAPPER_GO_TELEMETRY_MODE_FORMAT,
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
                    "name": "pasturestack:host-buildx-source-date-epoch",
                    "value": host_toolchain["HOST_BUILDX_SOURCE_DATE_EPOCH"],
                },
                {
                    "name": "pasturestack:host-buildx-source-filename",
                    "value": host_toolchain["HOST_BUILDX_SOURCE_FILENAME"],
                },
                {
                    "name": "pasturestack:host-buildx-source-url",
                    "value": host_toolchain["HOST_BUILDX_SOURCE_URL"],
                },
                {
                    "name": "pasturestack:host-buildx-source-sha256",
                    "value": host_toolchain["HOST_BUILDX_SOURCE_SHA256"],
                },
                {
                    "name": "pasturestack:host-buildx-security-patch-path",
                    "value": host_toolchain["HOST_BUILDX_SECURITY_PATCH_PATH"],
                },
                {
                    "name": "pasturestack:host-buildx-security-patch-sha256",
                    "value": host_toolchain["HOST_BUILDX_SECURITY_PATCH_SHA256"],
                },
                {
                    "name": "pasturestack:host-buildx-go-version",
                    "value": host_toolchain["HOST_BUILDX_GO_VERSION"],
                },
                {
                    "name": "pasturestack:host-buildx-go-platform",
                    "value": host_toolchain["HOST_BUILDX_GO_PLATFORM"],
                },
                {
                    "name": "pasturestack:host-buildx-go-filename",
                    "value": host_toolchain["HOST_BUILDX_GO_FILENAME"],
                },
                {
                    "name": "pasturestack:host-buildx-go-url",
                    "value": host_toolchain["HOST_BUILDX_GO_URL"],
                },
                {
                    "name": "pasturestack:host-buildx-go-sha256",
                    "value": host_toolchain["HOST_BUILDX_GO_SHA256"],
                },
                {
                    "name": "pasturestack:host-buildx-go-archive-version",
                    "value": host_toolchain["HOST_BUILDX_GO_ARCHIVE_VERSION"],
                },
                {
                    "name": "pasturestack:host-buildx-x-mod-version",
                    "value": host_toolchain["HOST_BUILDX_X_MOD_VERSION"],
                },
                {
                    "name": "pasturestack:host-buildx-binary-sha256",
                    "value": host_toolchain["HOST_BUILDX_BINARY_SHA256"],
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
        "dependencies": [
            {
                "ref": "pkg:github/PastureStack/ecr-credential-sync@3.1.0",
                "dependsOn": [component["bom-ref"] for component in components],
            },
            *[
                {"ref": component["bom-ref"], "dependsOn": []}
                for component in components
            ],
        ],
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
