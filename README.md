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
dependencies. The build image locks Docker CLI 29.7.2 and Buildx 0.36.1. A
checksum-locked Buildx patch removes its sole compiled dependency on the legacy
Docker module. The Ubuntu 26.04 base image is digest-pinned, and all direct APT
packages are locked to the official `20260808T000000Z` Ubuntu snapshot. Each
image records its resolved
`dpkg` inventory, and the build image records the installed GCC, Go, Docker,
and Buildx binaries.

```bash
VERSION_OVERRIDE=v3.1.0 ARCH=amd64 make build
VERSION_OVERRIDE=v3.1.0 ARCH=amd64 make test
```

These commands are local build and test targets. CI/CD publication and release
automation are outside this proof-of-concept scope.

GitHub Actions rebuilds the Linux binary and image twice without a build cache,
compares their hashes, and produces source, build-image, and runtime CycloneDX
SBOMs plus Trivy reports. The workflow never logs in to a registry, pushes an
image, creates a release, or changes a deployment.

The source SBOM intentionally inventories the preserved AWS SDK module even
though only its ECR and supporting packages are vendored. Module-level scanners
therefore report three S3 encryption-client advisories whose vulnerable package
is absent from both the vendor tree and the executable. The exact OpenVEX record
under `security/` documents that boundary; any new or unmatched source finding
fails the supply-chain workflow.

## Dependency version policy

- Go, Docker CLI, Buildx, GitHub Actions, Linux and Windows base images, and
  Trivy must use an exact version or immutable digest.
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
