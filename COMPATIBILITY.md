<!-- Modified by PastureStack contributors for independent maintenance and rebranding. -->

# Compatibility contracts

ECR Credential Sync uses PastureStack names for the repository, executable,
container image, and project-facing text. The POC exposes only neutral runtime
identifiers.

## Environment API contract

The environment API uses:

- `PLATFORM_URL`
- `PLATFORM_ACCESS_KEY`
- `PLATFORM_SECRET_KEY`

New Catalog templates inject those values directly. During an in-place
upgrade, the executable accepts the control plane's `CATTLE_URL`,
`CATTLE_ACCESS_KEY`, and `CATTLE_SECRET_KEY` compatibility variables only when
the matching neutral value is empty.

The minimal registry and credential client is implemented in
`internal/platformapi`. Its public contract is limited to the `registries` and
`registryCredentials` resource endpoints and the fields used by this service.

## Runtime assumptions

- AWS credentials use the AWS SDK credential provider chain.
- Shared profiles may be mounted under `/home/pasturestack/.aws`.
- `AWS_ECR_ENDPOINT_URL` optionally selects an HTTP(S) AWS-compatible ECR API
  endpoint. User information in the URL and non-HTTP(S) schemes are rejected.
- Registry resources are environment-scoped and exposed by the compatible
  control-plane API.
- The service performs one synchronization at startup and repeats every six
  hours.
- The health endpoint is `GET /ping`, on port `8080` unless `LISTEN_PORT` is
  set.
- The Windows image is built for `windows/amd64` on the Windows Server 2022
  (`ltsc2022`) container ABI. It uses the same executable and environment
  contract as the Linux image and runs as the built-in `ContainerUser`.
- Windows deployments require a Windows Server 2022-compatible container host
  and the `io.rancher.host.os=windows` compatibility scheduler label. The label
  is retained only because the legacy control plane uses it as a protocol
  identifier.

The control plane may still inject its historical environment API variables
when it creates an agent-backed service. The executable's bounded fallback
keeps that deployment path functional without changing the neutral public
contract. Those historical names and the `io.rancher.*` labels remain protocol
identifiers; they are not product branding.

This component has no user interface or runtime localization subsystem.
Operational log and flag strings remain English; no unsupported locale files
are included.
