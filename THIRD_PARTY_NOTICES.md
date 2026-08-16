# Third-party notices

The neutral `internal/platformapi` HTTP client was newly implemented for this
repository. Its registry and registry-credential JSON contracts were checked
against the historical `github.com/rancher/go-rancher` client at commit
`42edbb235a13e8739fe1354f01fb035172f0ed5f`.

No generated source from that client is included in the current tree or
claimed as PastureStack-authored work.

The source tree retains ten vendored legal files covering the runtime and test
dependencies present in the preserved upstream source:

- AWS SDK for Go: `LICENSE.txt` and `NOTICE.txt`.
- go-spew: `LICENSE`.
- go-ini: `LICENSE`.
- Gorilla WebSocket: `AUTHORS` and `LICENSE`.
- go-jmespath: `LICENSE`.
- Logrus: `LICENSE`.
- objx: `LICENSE.md`.
- try: `LICENSE`.

The container packaging gate copies all ten files under
`/licenses/third-party/` without replacing their text. Test-only dependencies
that are not linked into the runtime binary retain their source-tree legal
files where present.

The vendored Logrus `v0.10.0` source carries the narrowly scoped final
`v1.9.3` `Writer` scanner remediation from upstream commits
`6acd903758687c4a3db3c11701e6c414fcf1c1f7` and
`d40e25cd45ed9c6b2b66e6b97573a0413e4c23bd`; its existing MIT notice remains
unchanged.
