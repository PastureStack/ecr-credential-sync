<!-- Modified by PastureStack contributors for independent maintenance and rebranding. -->

# ECR Credential Sync

> PastureStack is an independent community effort to preserve, audit, and modernize the Rancher 1.6 ecosystem. It is not affiliated with or endorsed by Rancher Labs or SUSE.

**Upstream:** [`rancher/rancher-ecr-credentials`](https://github.com/rancher/rancher-ecr-credentials). This GitHub fork retains the upstream Git history, authorship, dates, and license notices unchanged; PastureStack maintenance is consolidated into one commit after the preserved upstream boundary.

ECR Credential Sync refreshes temporary Amazon Elastic Container Registry
credentials in a compatible legacy control-plane environment. It requests ECR
authorization tokens through the AWS SDK, finds the matching registry resource,
and updates its username and password. When `AUTO_CREATE=true`, it can create a
missing registry and credential resource.

This repository preserves the complete upstream Git history and contributor
record. PastureStack contributors claim authorship only for their own changes.
See [ORIGIN.md](ORIGIN.md) for provenance and [COMPATIBILITY.md](COMPATIBILITY.md)
for the neutral runtime contract.

## Runtime behavior

- Synchronizes credentials immediately after startup and then every six hours.
- Supports the default AWS credential provider chain and optional IAM role
  assumption through `AWS_ROLE_ARN`.
- Supports one or more account IDs through `AWS_ECR_REGISTRY_IDS`.
- Exposes `GET /ping` on port `8080` by default; set `LISTEN_PORT` to override it.
- Retries temporary ECR and environment API failures with a bounded
  incremental backoff. Successful requests do not wait or retry.
- Creates a missing credential record for an existing registry when
  `AUTO_CREATE=true`.
- Runs as the unprivileged numeric user `10001:10001`.
- Provides a `windows/amd64` LTSC 2022 variant that runs as `ContainerUser`.

## Configuration

| Variable | Required | Purpose |
| --- | --- | --- |
| `AWS_REGION` | Yes | AWS region containing the ECR registry. |
| `AWS_ACCESS_KEY_ID` | Depends on AWS environment | Static access key when no role, profile, or instance identity is available. |
| `AWS_SECRET_ACCESS_KEY` | Depends on AWS environment | Static secret key paired with the access key. |
| `AWS_SESSION_TOKEN` | No | Session token for temporary AWS credentials. |
| `AWS_PROFILE` | No | Profile from a mounted shared AWS configuration. |
| `AWS_ROLE_ARN` | No | IAM role to assume before requesting ECR tokens. |
| `AWS_ECR_REGISTRY_IDS` | No | Comma-separated AWS account IDs to synchronize. |
| `AWS_ECR_ENDPOINT_URL` | No | Alternate HTTP(S) ECR API endpoint for an AWS-compatible service or isolated validation. |
| `AUTO_CREATE` | No | Create a missing platform registry when set to `true`; default is `false`. |
| `LOG_LEVEL` | No | Logrus level such as `info`, `warn`, or `debug`. |
| `LISTEN_PORT` | No | Health endpoint port; default is `8080`. |
| `PLATFORM_URL` | Yes | Compatible environment API endpoint. |
| `PLATFORM_ACCESS_KEY` | Yes | Environment API access key. |
| `PLATFORM_SECRET_KEY` | Yes | Environment API secret key. |

New Catalog templates inject the `PLATFORM_*` variables documented in
[COMPATIBILITY.md](COMPATIBILITY.md). During an in-place upgrade, the
executable also accepts the control plane's compatibility variable names only
when the corresponding neutral value is absent.

Use least-privilege AWS permissions that allow ECR authorization token requests.
If a shared AWS configuration is mounted under `/home/pasturestack/.aws`,
protect the mount as credential-bearing data.

## Container example

```bash
docker run --rm \
  -e AWS_REGION=us-east-1 \
  -e AWS_ACCESS_KEY_ID \
  -e AWS_SECRET_ACCESS_KEY \
  -e PLATFORM_URL \
  -e PLATFORM_ACCESS_KEY \
  -e PLATFORM_SECRET_KEY \
ghcr.io/pasturestack/ecr-credential-sync:v3.1.0
```

Windows Server 2022 hosts use the separately versioned
`v3.1.2-windows-ltsc2022` image. The Catalog selects it only for Windows
environments; Linux deployments continue to use `v3.1.0`.

Do not publish a mutable `latest` tag. Deployment examples and Catalog
templates use immutable semantic version tags; release digests are retained
only in internal validation evidence.

## Local build and test

The project uses a containerized Go 1.26.6 build environment with vendored
dependencies. The build image compiles Docker CLI 29.7.2 from its checksum- and
commit-locked official source with Go 1.26.6, and records both the binary hash
and embedded Go build information. It also locks Buildx 0.36.1 and installs
`jq` `1.8.1-4ubuntu2` as the fail-closed Dapper identity metadata parser. A
checksum-locked Buildx patch removes its sole compiled dependency on the legacy
Docker module.
The host build client is installed as Buildx `v0.36.1` by the repository-owned
`scripts/install-locked-host-buildx` verifier. GitHub Actions gives the verified
release binary an empty, caller-owned mode-`0700` run root under `$HOME`, then
copies it into a separate run-owned `DOCKER_CONFIG` at
`cli-plugins/docker-buildx`. `DAPPER_BUILDX_COMMAND` remains bound to the
installer-returned binary, while every Dapper builder uses BuildKit `v0.32.2` from
`moby/buildkit:v0.32.2@sha256:28a898719c18a33f4e8000685287fa36fd0dd9560c6440227d3a732d79bb41d8`.
These host-side locks are recorded in `toolchain/host-build-toolchain.lock` and
the source SBOM. The lock also binds the official Linux amd64 Buildx release
asset (`sha256:48af8a397ebd60178778bf63611dbcebe5f5e7a9be90eb9147b24b9587455778`)
and `checksums.txt`
(`sha256:abeea7a52865e60e1af4995d2449cdbaca762dc99689a829f15f0fd760766413`)
to their exact HTTPS URLs. The Ubuntu 26.04 base image is digest-pinned, and
all direct APT packages are locked to the official `20260808T000000Z` Ubuntu
snapshot. Each image records its resolved `dpkg` inventory, and the build image
records the installed GCC, Go, Docker, Buildx, and `jq` binaries. The source
SBOM records the Ubuntu APT lock digest, snapshot, and exact Dapper `jq` version;
the Dapper image SBOM independently inventories the installed package.

Runtime packaging uses an isolated, run-owned `docker-container` builder with
that same pinned BuildKit image. It keeps `rewrite-timestamp=true` and
`compatibility-version=20`, exports to a CreateNew Docker archive, records and
reads back the archive SHA-256, and only then performs an explicit
`docker image load`. The Buildx IID is checked as the top-level config digest
when that field exists, or as the manifest digest when Buildx uses its documented
fallback. The loaded daemon image ID must match every available metadata config
source or the exported manifest digest, and the selected daemon-ID mode is
recorded before the package is accepted. At least one metadata config source
must exist, and all present sources must agree. Cleanup is limited to the exact
run-owned archive, Buildx instance, builder container, state volume, and—on a
failed load or identity gate—the newly created image reference; it never prunes
shared builder state.

If a development packaging host does not already provide the locked Buildx,
install the verified release binary only under an empty, caller-owned run root:

```bash
mkdir -m 0700 "$RUN_ROOT/locked-host-buildx"
export DAPPER_BUILDX_COMMAND="$(bash scripts/install-locked-host-buildx "$RUN_ROOT/locked-host-buildx")"
"$DAPPER_BUILDX_COMMAND" version
```

The helper verifies the official checksum file, binary hash, version, and
commit before returning the executable path. It refuses existing, symlinked,
non-canonical, system, and global Docker plugin destinations. Every path
ancestor must be owned by root or the caller; group- or world-writable
ancestors must use the sticky bit. The helper binds subsequent operations to
the validated run-root identity and never writes to `/usr` or the user's global
Docker CLI plugin directory. GitHub Actions uses this same verifier and checks
the copied plugin's hash, mode, version, and commit before persisting its isolated
`DOCKER_CONFIG`. Its always-run cleanup removes only the two exact run roots
after their paths, ownership, modes, and directory identities are revalidated.
The development packaging harness must use this exact command for builder
create, inspect, and removal, and pass it to `make` with the pinned builder.

```bash
VERSION_OVERRIDE=v3.1.0 ARCH=amd64 make build
VERSION_OVERRIDE=v3.1.0 ARCH=amd64 make test
```

These commands are local build and test targets. CI/CD publication and release
automation are outside this proof-of-concept scope.

GitHub Actions rebuilds the Linux binary and image twice without a build cache,
using two isolated digest-pinned BuildKit builders. Each build applies Buildx's
exact IID selection rule: the top-level configuration digest when present, or
the exported manifest digest otherwise. It requires the metadata configuration
sources to agree, then accepts the loaded Docker daemon ID only when it equals
that configuration digest or the exported manifest digest and records which
engine mode was observed. It then compares the two image IDs,
configuration digests, complete RootFS DiffID lists, creation timestamps,
binaries, and embedded manifests. The workflow produces source, build-image,
and runtime CycloneDX SBOMs plus Trivy reports. It never logs in to a registry,
pushes an image, creates a release, or changes a deployment.

The source SBOM intentionally inventories the preserved AWS SDK module even
though only its ECR and supporting packages are vendored. Module-level scanners
therefore report three S3 encryption-client advisories whose vulnerable package
is absent from both the vendor tree and the executable. The exact OpenVEX record
under `security/` documents that boundary; any new or unmatched source finding
fails the supply-chain workflow.

## Dependency version policy

- Go, Docker CLI, Buildx, BuildKit, GitHub Actions, Linux and Windows base
  images, and Trivy must use an exact version, commit, or immutable digest.
- The Ubuntu snapshot and every direct APT package version are one atomic lock;
  update them together and verify the resolved `dpkg` manifests.
- Operational container references remain plain semantic version tags. Do not
  use `latest`, branch tags, digest suffixes in user-facing image fields, or
  product-specific suffixes in version numbers.
- A version refresh is complete only after unit and race tests, reproducibility
  checks, SBOM generation, vulnerability scanning, license checks, and the
  downstream Catalog compatibility review all pass.

## Security and support

Read [SECURITY.md](SECURITY.md) before deployment. This compatibility component
handles AWS and platform credentials and must not be exposed as a public
general-purpose service.

The affiliation disclaimer at the top of this README applies to all builds and
distributions. Use the PastureStack repository issue tracker for
project-specific reports, without including credentials, authorization tokens,
or private URLs.

## License

The project remains licensed under the existing [Apache License 2.0](LICENSE).
The license was not replaced by PastureStack. Vendored dependencies retain their
own copyright, license, and notice files; redistribution must preserve them.
