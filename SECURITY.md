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
  uses Go 1.27.0, Docker CLI 29.7.2, and the checksum-patched Buildx 0.36.1
  source. Its identity resolver uses `jq` `1.8.1-4ubuntu2`, installed from the
  same dated Ubuntu snapshot and verified against the direct package lock.
- The host Buildx client is `v0.36.1`, built twice by the repository-owned
  `scripts/install-locked-host-buildx` verifier inside an empty, caller-owned
  mode-`0700` run root below `$HOME`. CI copies the byte-identical binary into a
  separate run-owned `DOCKER_CONFIG` at `cli-plugins/docker-buildx`, verifies
  its hash, owner, mode, version, and commit, and binds
  `DAPPER_BUILDX_COMMAND` to the installer-returned binary. Dapper builders use
  BuildKit `v0.32.2` from
  `moby/buildkit:v0.32.2@sha256:28a898719c18a33f4e8000685287fa36fd0dd9560c6440227d3a732d79bb41d8`.
  The source gate rejects mutable BuildKit fallbacks.
- Development packaging may use `scripts/install-locked-host-buildx` only when
  the host plugin does not match the lock. The helper downloads the exact
  Buildx source (`sha256:fb28b5c2a198d05482f0656dfb7ee161240a904e36697bf7108e5d517f23854b`)
  and Go 1.27.0 archive
  (`sha256:675c26c449cbb18fc24b74650de1eabbae6e16f64326fd85a283fb3b58280685`),
  applies the repository patch
  (`sha256:a29fdda204d592d0d46ed32b5fdc4336a7b3a7276301836d0a75b9386ca780f7`),
  and performs two vendor-only, `GOPROXY=off` builds. It requires identical
  dependency lists and binaries, verifies result
  `sha256:6bd2cb8809abf99fd2a0acaf50a51b85e9a4b254cc9bfede6305e85f42c59eb2`,
  Buildx `v0.36.1`, commit `1d8dde89b8aba914e05e45366770736fea1fd690`,
  `github.com/moby/go-archive v0.3.0`, and `golang.org/x/mod v0.40.0`. It writes
  only below an empty,
  canonical, caller-owned run root whose ancestors are root- or caller-owned;
  any group- or world-writable ancestor must be sticky. Later operations stay
  bound to that validated directory identity. The helper never installs into a
  system directory or global Docker CLI plugin directory. Always-run cleanup
  revalidates the exact two run-root paths, identities, owners, and mode `0700`
  before deleting only those directories. As of 2026-08-20, the latest signed
  upstream Buildx release remains `v0.36.1`; its official binary contains
  `go-archive v0.2.1` and `x/mod v0.38.0`, and no signed fixed release exists.
  The same locked upgrade is therefore required for both host and Dapper
  binaries; these CVEs must not be suppressed with OpenVEX.
- Every Dapper export records its manifest digest separately. The Buildx IID
  must equal the top-level configuration digest when that metadata field is
  present; otherwise it must equal the manifest digest selected by Buildx's
  fallback rule. At least one metadata configuration source is required, and
  all present sources must agree. The loaded Docker daemon image ID must equal either that
  configuration digest or the exported manifest digest; the observed engine
  mode is recorded. Missing, malformed, or conflicting identity evidence fails
  closed.
- Runtime images are built by an exact run-owned `docker-container` builder
  using the locked BuildKit image. The reproducible exporter writes a new
  Docker archive without daemon unpack/load options, verifies the archive hash,
  then calls `docker image load` explicitly. The loaded image must pass the same
  Buildx IID-mode, metadata, daemon configuration-ID, label, and non-root-user
  checks. Failure cleanup is
  restricted to the exact run-owned archive, builder, container, state volume,
  and identity-bound image; broad builder or image pruning is forbidden.
- The build does not download the historical Dapper binary or use its retired
  upstream build image.
- Go telemetry is disabled with the official `go telemetry off` command before
  the first ordinary Go invocation. Its mode file is then canonicalized to the
  exact `off YYYY-MM-DD` format with the UTC date and metadata derived from
  `SOURCE_DATE_EPOCH`; an OS `GOTELEMETRY` variable is not accepted as a
  control. The same repository-owned verifier runs after every Go-bearing
  layer and in final-image evidence, requiring the derived mode and directory
  values and rejecting local, upload, debug, counter, weekends, or `.count`
  state.
- The Windows variant uses the Microsoft Nano Server LTSC 2022 base, runs as
  `ContainerUser`, and contains only the cross-compiled service executable
  above that base. Validate the host build against the Windows Server 2022
  container compatibility matrix before production rollout.
- Runtime distributions include all 27 vendored license, notice, and
  author files in addition to the root license and attribution documents.
- The source SBOM is generated from `vendor/modules.txt`; source vulnerabilities
  are not suppressed with OpenVEX and every finding fails the gate. The
  disposable builder may assess only unfixed `linux-libc-dev` findings because
  the package supplies headers rather than the vulnerable kernel implementation
  and is absent from the runtime image. That exact-package assessment expires
  on 2026-09-15; every other builder finding fails the gate.
- Do not add broad vulnerability, secret, or misconfiguration ignores.

## Reporting

Report vulnerabilities privately to the PastureStack maintainers. Do not place
credentials, tokens, private API endpoints, account IDs, or unredacted logs in
an issue.
