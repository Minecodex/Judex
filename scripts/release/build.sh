#!/usr/bin/env bash
# Judex release builder (docs/plans/v1/07 §8 / 11 §5). Usage: build.sh [VERSION]
set -euo pipefail
VERSION="${1:-0.1.0-dev}"
OUT="dist/release/${VERSION}"
mkdir -p "${OUT}/cli" "${OUT}/skill"
export CGO_ENABLED=0
build_one() {
  local goos="$1" goarch="$2" ext="$3"
  local name="judex-${goos}-${goarch}-${VERSION}${ext}"
  GOOS="${goos}" GOARCH="${goarch}" go build -trimpath \
    -ldflags "-s -w -X github.com/kakj-go/Judex/internal/version.Version=${VERSION}" \
    -o "${OUT}/cli/${name}" ./cmd/judex
  echo "built ${name}"
}
build_one windows amd64 .exe
build_one windows arm64 .exe
build_one linux amd64 ""
build_one linux arm64 ""
build_one darwin amd64 ""
build_one darwin arm64 ""
go build -trimpath -ldflags "-X github.com/kakj-go/Judex/internal/version.Version=${VERSION}" \
  -o "${OUT}/judex-server" ./cmd/judex-server
python - "$OUT" "$VERSION" <<'PY'
import os, sys, zipfile, hashlib
out, version = sys.argv[1], sys.argv[2]
with zipfile.ZipFile(os.path.join(out, "skill", f"judex-skill-{version}.zip"), "w", zipfile.ZIP_DEFLATED) as z:
    for root, _, files in os.walk("skills/judex"):
        for f in files:
            p = os.path.join(root, f)
            z.write(p, os.path.relpath(p, "skills/judex"))
lines = []
for root, _, files in os.walk(out):
    for f in files:
        if f == "checksums.txt":
            continue
        p = os.path.join(root, f)
        h = hashlib.sha256(open(p, "rb").read()).hexdigest()
        lines.append(f"{h}  {os.path.relpath(p, out)}")
open(os.path.join(out, "checksums.txt"), "w", newline="\n").write("\n".join(lines) + "\n")
manifest = {
    "version": version, "protocolVersion": "1",
    "components": ["server", "web", "cli", "skill", "chart", "openapi"],
    "cliPlatforms": ["windows/amd64", "windows/arm64", "linux/amd64", "linux/arm64", "darwin/amd64", "darwin/arm64"],
}
import json
open(os.path.join(out, "manifest.json"), "w").write(json.dumps(manifest, indent=2) + "\n")
print("checksums + manifest written")
PY
echo "release artifacts in ${OUT}"
