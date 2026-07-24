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

- Build and runtime images use Ubuntu 26.04 and Go 1.26.5.
- The build does not download the historical Dapper binary or use its retired
  upstream build image.
- The Windows container build path remains disabled until a Windows host
  compatibility matrix is restored.
- Runtime distributions include all ten tracked vendored license, notice, and
  author files in addition to the root license and attribution documents.
- Do not add broad vulnerability, secret, or misconfiguration ignores.

## Reporting

Report vulnerabilities privately to the PastureStack maintainers. Do not place
credentials, tokens, private API endpoints, account IDs, or unredacted logs in
an issue.
