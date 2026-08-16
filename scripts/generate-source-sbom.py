#!/usr/bin/env python3
"""Generate a deterministic CycloneDX inventory from the legacy vendor lock."""

from __future__ import annotations

import argparse
import hashlib
import json
from pathlib import Path
import uuid


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


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--lock", type=Path, default=Path("vendor.conf"))
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()

    lock_text = args.lock.read_text(encoding="utf-8")
    lock_bytes = lock_text.replace("\r\n", "\n").replace("\r", "\n").encode("utf-8")
    dependencies = parse_vendor_lock(args.lock)
    lock_digest = hashlib.sha256(lock_bytes).hexdigest()
    serial = uuid.uuid5(uuid.NAMESPACE_URL, f"pasturestack:ecr-credential-sync:{lock_digest}")

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
                {"name": "pasturestack:vendor-lock-sha256", "value": lock_digest}
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
