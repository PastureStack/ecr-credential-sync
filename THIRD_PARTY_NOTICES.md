# Third-party notices

The neutral `internal/platformapi` HTTP client was newly implemented for this
repository. Its registry and registry-credential JSON contracts were checked
against the historical `github.com/rancher/go-rancher` client at commit
`42edbb235a13e8739fe1354f01fb035172f0ed5f`.

No generated source from that client is included in the current tree or
claimed as PastureStack-authored work.

The source tree retains 27 vendored legal files covering the runtime and test
dependencies selected by `vendor/modules.txt`:

- AWS SDK for Go v2 modules: their `LICENSE.txt`, `NOTICE.txt`, and internal
  singleflight license files.
- AWS Smithy for Go: `LICENSE`, `NOTICE`, and its internal singleflight license.
- Logrus: `LICENSE`.
- Testify, objx, and their internal difflib and spew packages: `LICENSE` files.
- YAML v3: `LICENSE` and `NOTICE`.
- Go `x/sys`: `LICENSE`.

The container packaging gate copies all 27 files under
`/licenses/third-party/` without replacing their text. Test-only dependencies
that are not linked into the runtime binary retain their source-tree legal
files where present.
