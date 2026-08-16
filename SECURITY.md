<!-- Modified by PastureStack contributors for independent maintenance and rebranding. -->

# Security Notes

ECR Credential Sync is a credential-bearing compatibility helper. It handles temporary
ECR passwords, AWS credentials, and environment-scoped platform API
credentials. Do not expose it directly to untrusted users or public networks.

## Credential handling

- Use least-privilege AWS IAM permissions that allow only the required ECR
  authorization token operation and optional role assumption.
- Treat `PLATFORM_ACCESS_KEY`, `PLATFORM_SECRET_KEY`, AWS environment variables, and
  `/home/pasturestack/.aws` as secrets.
- Do not enable debug logging unless the resulting logs are protected as
  credential-bearing operational data.
- ECR authorization tokens are never logged. Malformed decoded tokens are
  reported only with a redaction marker and decoded length.
- Nil and incomplete AWS authorization responses are handled without panics.

## Retry boundary

AWS ECR and environment API failures use ten bounded attempts with an
incremental ten-second backoff. Waiting occurs only between failed attempts. A
successful request returns immediately. The policy is injectable in tests so
the test suite never waits on production delays.

## Compatibility boundary

- Registry and registry-credential API field behavior remains compatible.
- `PLATFORM_URL`, `PLATFORM_ACCESS_KEY`, and `PLATFORM_SECRET_KEY` are required
  contracts.
- The runtime uses numeric user and group `10001:10001`; it does not require
  privileged mode, host networking, the Docker socket, or host filesystem
  access.
- The minimal client under `internal/platformapi` sends only the required
  registry and credential operations. See [COMPATIBILITY.md](COMPATIBILITY.md).

## Build boundary

- Build and runtime images use the digest-pinned Ubuntu 26.04 base. The build
  uses Go 1.26.6, Docker CLI 29.7.2, and the checksum-patched Buildx 0.36.1
  source.
- The host Buildx client is `v0.36.1`, installed by
  `docker/setup-buildx-action@bb05f3f5519dd87d3ba754cc423b652a5edd6d2c`
  (`v4.2.0`). Dapper builders use BuildKit `v0.32.2` from
  `moby/buildkit:v0.32.2@sha256:28a898719c18a33f4e8000685287fa36fd0dd9560c6440227d3a732d79bb41d8`.
  The source gate rejects mutable BuildKit fallbacks.
- Development packaging may use `scripts/install-locked-host-buildx` only when
  the host plugin does not match the lock. The helper downloads the exact
  official Linux amd64 asset and checksum file over HTTPS, verifies both
  SHA-256 digests (`48af8a397ebd60178778bf63611dbcebe5f5e7a9be90eb9147b24b9587455778`
  and `abeea7a52865e60e1af4995d2449cdbaca762dc99689a829f15f0fd760766413`),
  then verifies Buildx `v0.36.1` and commit
  `1d8dde89b8aba914e05e45366770736fea1fd690`. It writes only below an empty,
  canonical, caller-owned run root and never installs into a system directory
  or global Docker CLI plugin directory.
- Every Dapper export records its manifest digest separately. Its Buildx IID
  must be a valid configuration JSON digest and must equal the loaded Docker
  daemon image ID. Any non-null config digest emitted in Buildx metadata must
  also match; missing, malformed, or conflicting required identity evidence
  fails closed.
- The build does not download the historical Dapper binary or use its retired
  upstream build image.
- The Windows variant uses the Microsoft Nano Server LTSC 2022 base, runs as
  `ContainerUser`, and contains only the cross-compiled service executable
  above that base. Validate the host build against the Windows Server 2022
  container compatibility matrix before production rollout.
- Runtime distributions include all ten tracked vendored license, notice, and
  author files in addition to the root license and attribution documents.
- The source OpenVEX record is bound to exact dependency PURLs and is revalidated by package-absence and regression gates on every build. The disposable builder may assess only unfixed `linux-libc-dev` findings because the package supplies headers rather than the vulnerable kernel implementation and is absent from the runtime image. That exact-package assessment expires on 2026-09-15; every other builder finding fails the gate.
- Do not add broad vulnerability, secret, or misconfiguration ignores.

## Reporting

Report vulnerabilities privately to the PastureStack maintainers. Do not place
credentials, tokens, private API endpoints, account IDs, or unredacted logs in
an issue.
