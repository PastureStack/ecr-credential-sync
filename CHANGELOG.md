<!-- Modified by PastureStack contributors for independent maintenance and rebranding. -->

# ChangeLog

> This upstream changelog is preserved for historical accuracy. Product names
> below describe the environment at the time of each original release. New
> PastureStack maintenance changes are recorded in the preserved Git history.

## v3.1.0 (2026/07/25)

- Replace the broad generated control-plane client with a bounded minimal
  registry and registry-credential client.
- Add full create and update lifecycle coverage against mock ECR and
  environment APIs.
- Retry both ECR and environment API failures without logging credentials.
- Add an optional validated ECR endpoint override for isolated validation and
  AWS-compatible services.
- Run the Ubuntu 26.04 runtime as unprivileged user `10001:10001`.
- Package all tracked vendored license, notice, and author files.
- Build and test with Go 1.27.0 and checksum-verified tool archives.
- Replace the end-of-life AWS SDK for Go v1 with the official AWS SDK for Go v2
  ECR, STS, configuration, and credential modules.
- Upgrade `golang.org/x/sys` to `v0.47.0`, closing the source-SBOM finding for
  CVE-2026-39824.

## v1.2.0 (2017/03/12)

* Add HTTP healthcheck for container (default: :8080/ping, configurable with `LISTEN_PORT` envvar)
* Support multiple ECR registries with `AWS_ECR_REGISTRY_IDS` envvar
* Support auto creating an ECR registry in the control plane using the `AUTO_CREATE` envvar (false by default)
* Support Assuming IAM Roles using the `AWS_ROLE_ARN` envvar.

## v1.1.0 (2016/07/21)

* Support IAM Instance Profiles for AWS API credentials

## v1.0.1 (2016/03/22)

* [bug] - don't exit process when error connecting to the control-plane API

## v1.0.0 (2016/03/12)

* Initial Release
