import hashlib, http.server, json, subprocess, threading, ssl, tempfile, os
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
BINARY = str(ROOT / "evidence/docker-cli")
assert hashlib.sha256(Path(BINARY).read_bytes()).hexdigest() == (ROOT / "evidence/docker-cli.sha256").read_text().strip()
def raw(x): return json.dumps(x, separators=(",", ":")).encode()
def digest(x): return "sha256:" + hashlib.sha256(x).hexdigest()
config = raw({"architecture":"amd64","os":"linux","rootfs":{"type":"layers","diff_ids":[]},"config":{}})
def manifest(oci=False, evil=False):
    return raw({"schemaVersion":2, "mediaType":"application/vnd.oci.image.manifest.v1+json" if oci else "application/vnd.docker.distribution.manifest.v2+json",
                "config":{"mediaType":"application/vnd.oci.image.config.v1+json" if oci else "application/vnd.docker.container.image.v1+json","size":len(config),"digest":digest(config)},
                "layers":[], "annotations":{"proof":"attacker" if evil else "trusted"}})
schema = manifest()
index = raw({"schemaVersion":2,"mediaType":"application/vnd.docker.distribution.manifest.list.v2+json","manifests":[{"mediaType":"application/vnd.docker.distribution.manifest.v2+json","size":len(schema),"digest":digest(schema),"platform":{"os":"linux","architecture":"amd64"}}]})
state = {}
requests_seen = []
class Handler(http.server.BaseHTTPRequestHandler):
    def log_message(self, *args): pass
    def do_GET(self):
        requests_seen.append(self.path)
        if self.path == "/v2/": data, media = b"{}", "application/json"
        elif "/manifests/" in self.path:
            requested = self.path.rsplit("/", 1)[1]
            data = state["body"] if requested == state["requested"] else state.get("child", schema)
            media = json.loads(data)["mediaType"]
            # Forge the expected digest using fixture-owned values, never request
            # bytes. The proof tests the CLI's content validation, not a registry
            # header parser, and must not make the local server header-injectable.
            advertised_digest = state["requested"] if requested == state["requested"] else digest(schema)
        elif "/blobs/" in self.path: data, media = config, "application/octet-stream"
        else: self.send_error(404); return
        self.send_response(200)
        self.send_header("Content-Type", media)
        self.send_header("Content-Length", str(len(data)))
        if "/manifests/" in self.path:
            self.send_header("Docker-Content-Digest", advertised_digest)
        self.end_headers(); self.wfile.write(data)
fixture = tempfile.TemporaryDirectory(prefix="docker-manifest-proof-")
config_dir = Path(fixture.name) / "docker-config"
config_dir.mkdir()
environment = dict(os.environ, DOCKER_CONFIG=str(config_dir))
cert = Path(fixture.name) / "cert.pem"
key = Path(fixture.name) / "key.pem"
subprocess.run(["openssl","req","-x509","-newkey","rsa:2048","-nodes","-keyout",str(key),"-out",str(cert),"-subj","/CN=localhost","-days","1","-addext","subjectAltName=IP:127.0.0.1"],check=True,capture_output=True)
server = http.server.ThreadingHTTPServer(("127.0.0.1",0), Handler)
tls = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
tls.minimum_version = ssl.TLSVersion.TLSv1_2
tls.load_cert_chain(cert, key)
server.socket = tls.wrap_socket(server.socket, server_side=True)
threading.Thread(target=server.serve_forever, daemon=True).start()
cases = [("valid_schema2",schema,schema,None,True),
         ("forged_schema2",schema,manifest(evil=True),None,False),
         ("valid_oci",manifest(True),manifest(True),None,True),
         ("forged_oci",manifest(True),manifest(True,True),None,False),
         ("valid_list",index,index,schema,True),
         ("forged_list_parent",index,raw({"schemaVersion":2,"mediaType":"application/vnd.docker.distribution.manifest.list.v2+json","manifests":[]}),schema,False),
         ("forged_list_child",index,index,manifest(evil=True),False)]
results = []
try:
    for name, expected, body, child, accepted in cases:
        state.clear(); state.update(requested=digest(expected),body=body,child=child or schema)
        requests_seen.clear()
        result = subprocess.run([BINARY,"manifest","inspect","--insecure",f"127.0.0.1:{server.server_port}/library/test@{digest(expected)}"],capture_output=True,text=True,timeout=30,env=environment)
        passed = (result.returncode == 0) if accepted else (result.returncode != 0 and not result.stdout.strip() and not any("/blobs/" in p for p in requests_seen) and ("manifest verification failed" in result.stderr or "unsupported manifest format" in result.stderr))
        results.append({"case":name,"passed":passed,"accepted":result.returncode==0,"expectedAccept":accepted,
                        "returnCode":result.returncode,"stderr":result.stderr.strip(),
                        "requestPaths":list(requests_seen)})
        if not passed: raise AssertionError(results[-1])
finally:
    server.shutdown()
    print(json.dumps({"binarySha256":hashlib.sha256(Path(BINARY).read_bytes()).hexdigest(),"sourceCommit":"a7dcaa6fdb6ed04aacbfdc76357fdae01605609e","cases":results},indent=2))
