"""Provision the user's already-authorized key via stdin, never a file/log."""
import json
import pathlib
import re
import subprocess
import sys

namespace, run_id, uid, transcript = sys.argv[1:]
if namespace != run_id or not namespace.startswith("judex-ui-realhost-"):
    raise SystemExit("namespace ownership required")
state = json.loads(subprocess.check_output(["kubectl", "get", "namespace", namespace, "-o", "json"]))
if state["metadata"]["uid"] != uid or state["metadata"]["labels"].get("judex.dev/test-run") != run_id:
    raise SystemExit("namespace ownership changed")
keys = []
for line in pathlib.Path(transcript).read_text(encoding="utf-8").splitlines():
    try:
        item = json.loads(line)
    except ValueError:
        continue
    payload = item.get("payload", {})
    message = ""
    if item.get("type") == "response_item" and payload.get("role") == "user":
        message = "\n".join(c.get("text", "") for c in payload.get("content", []) if isinstance(c, dict))
    elif item.get("type") == "event_msg" and payload.get("type") == "user_message":
        message = payload.get("message", "")
    if "glm" in message.lower():
        keys.extend(re.findall(r"[0-9a-fA-F]{32}\.[A-Za-z0-9_-]{12,128}", message))
if not keys:
    raise SystemExit("authorized GLM key not found in the specified conversation")
secret = {"apiVersion": "v1", "kind": "Secret", "metadata": {"name": "real-model-key", "namespace": namespace,
          "labels": {"judex.dev/test-run": run_id}}, "stringData": {"JUDEX_GLM_CODING_KEY": keys[-1]}}
result = subprocess.run(["kubectl", "create", "-f", "-"], input=json.dumps(secret).encode(), capture_output=True)
if result.returncode:
    raise SystemExit("model Secret could not be created; no key was written to disk")
print("Authorized model Secret created in the owned test namespace.")
