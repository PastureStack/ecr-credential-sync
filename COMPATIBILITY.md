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

Catalog templates must inject those values without relying on retired
product-specific agent labels.

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
- The Windows image path is disabled pending a validated compatibility matrix.

The compatible Catalog command maps the control plane's historical injected
environment API variables to `PLATFORM_*` before starting the binary. Those
historical names and the `io.rancher.*` labels remain protocol identifiers;
they are not product branding.

This component has no user interface or runtime localization subsystem.
Operational log and flag strings remain English; no unsupported locale files
are included.
