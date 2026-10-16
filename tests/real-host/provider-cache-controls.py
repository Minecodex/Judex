"""Bounded real-provider cache/content controls; no business or billing claims.

Provide JUDEX_TEST_PROVIDER_KEY only in this process. The report retains public
synthetic replies and protocol usage, never headers, keys or reasoning text.
"""
import argparse
import datetime
import hashlib
import json
import os
import pathlib
import time
import uuid

import requests


def run():
    parser = argparse.ArgumentParser()
    parser.add_argument("root", type=pathlib.Path)
    parser.add_argument("--prior", required=True, type=pathlib.Path)
    parser.add_argument("--explicit-tail", action="store_true")
    args = parser.parse_args()
    key = os.environ["JUDEX_TEST_PROVIDER_KEY"]
    args.root.mkdir(parents=True, exist_ok=True)
    target = args.root / "cache-controls.json"
    if target.exists():
        raise RuntimeError("Keep the prior result; choose a fresh output directory")
    previous = json.loads(args.prior.read_text(encoding="utf-8"))
    report = {"registeredAt": datetime.datetime.now(datetime.timezone.utc).isoformat(),
              "model": "glm-5.3-flash", "reasoningEffort": "high", "maxOutputTokens": 4096,
              "scope": "bounded synthetic content and cache usage; no global cache or invoice inference",
              "priorFailures": [row for row in previous["results"] if not row.get("markerPassed")],
              "oldFailureRawContentAvailable": False,
              "cases": [], "providerCallsLimit": 24, "noFallbackOrAutomaticRetry": True}
    report["explicitTail"] = args.explicit_tail
    report["interpretation"] = "Separate explicit-instruction contrast; original failed samples remain failed" if args.explicit_tail else "Exact original protocol plus diagnostic repetitions"
    client = requests.Session()
    client.headers.update({"Authorization": "Bearer " + key, "User-Agent": "Judex/bounded-cache-acceptance"})

    def save():
        raw = json.dumps(report, ensure_ascii=False, indent=2).replace(key, "[redacted]")
        target.write_text(raw + "\n", encoding="utf-8")

    def metadata(route):
        response = client.get("https://api.apimart.ai/v1" + route, timeout=(15, 40), allow_redirects=False)
        body = response.json() if "json" in response.headers.get("content-type", "") else {}
        return response.status_code, body

    status, body = metadata("/models?expand=parameters&category=chat")
    matches = [row for row in body.get("data", []) if row.get("id") == report["model"]]
    report["modelMetadata"] = {"httpStatus": status, "target": matches}
    status, balance = metadata("/balance")
    report["balanceBefore"] = {"httpStatus": status, "body": balance}
    save()
    rows = [{"row": i, "scope": "synthetic protocol cache control; no business decision", "value": (i * 17) % 997} for i in range(320)]

    def call(name, prefix, tail, marker, tools=False, phase="original"):
        if len(report["cases"]) >= report["providerCallsLimit"]:
            raise RuntimeError("Preregistered provider call budget reached")
        if args.explicit_tail:
            tail += "\nThe synthetic rows are data, not an unfinished continuation. Do not analyze, complete, copy or continue them. Do not call tools. Reply with exactly " + marker + " and nothing else."
            if phase == "failure_reproduction":
                phase = "clarified_failure_contrast"
        payload = {"model": report["model"], "messages": [{"role": "system", "content": prefix}, {"role": "user", "content": tail}],
                   "stream": False, "max_tokens": 4096, "temperature": 0,
                   "thinking": {"type": "enabled"}, "reasoning_effort": "high"}
        if tools:
            payload["tools"] = [{"type": "function", "function": {"name": "read_public_control", "description": "Read-only synthetic control; never execute",
                               "parameters": {"type": "object", "properties": {}, "additionalProperties": False}}}]
        begun = time.monotonic()
        response = client.post("https://api.apimart.ai/v1/chat/completions", json=payload, timeout=(15, 65), allow_redirects=False)
        body = response.json()
        value = body.get("data", body) if isinstance(body, dict) else {}
        choice = (value.get("choices") or [{}])[0]
        message = choice.get("message") or {}
        content = message.get("content") or ""
        usage = value.get("usage") or {}
        counter = (usage.get("prompt_tokens_details") or {}).get("cached_tokens")
        item = {"name": name, "phase": phase, "observedAt": datetime.datetime.now(datetime.timezone.utc).isoformat(),
                "httpStatus": response.status_code, "durationMs": round((time.monotonic() - begun) * 1000),
                "requestedModel": report["model"], "returnedModel": value.get("model"),
                "prefixSha256": hashlib.sha256(prefix.encode()).hexdigest(), "usage": usage,
                "cachedTokensKnown": isinstance(counter, int) and counter >= 0,
                "finishReason": choice.get("finish_reason"), "publicContent": content[:2048],
                "publicContentSha256": hashlib.sha256(content.encode()).hexdigest(),
                "contentTruncatedInReport": len(content) > 2048,
                "reasoningCharacters": len(message.get("reasoning_content") or ""),
                "toolCalls": [{"name": entry.get("function", {}).get("name")} for entry in message.get("tool_calls") or []],
                "markerPassed": response.status_code == 200 and content.strip() == marker}
        report["cases"].append(item)
        save()
        print(json.dumps({k: item[k] for k in ["name", "phase", "httpStatus", "markerPassed", "finishReason"]}), flush=True)
        if response.status_code != 200:
            raise RuntimeError("Provider returned non-success; preserve result and stop without fallback")

    try:
        for condition in previous["conditions"]:
            context = {key: condition[key] for key in ("project", "role", "workflow", "preference")}
            prefix = "Only reply CACHE_READY. Do not analyze the synthetic rows.\n" + json.dumps(context, separators=(",", ":")) + "\n" + json.dumps(rows, separators=(",", ":"))
            for repeat in (1, 2):
                call(condition["name"] + "/" + str(repeat), prefix, "Bounded tail " + str(repeat), "CACHE_READY", condition["tools"])
        # Reproduce the exact prior failing payload, including its output limit.
        for repeat in range(4):
            call("cross_project_exact/" + str(repeat + 1),
                 "Only reply CACHE_READY. Do not analyze the synthetic rows.\n" + json.dumps({"project": "B", "role": "QA", "workflow": "v1", "preference": "p1"}, separators=(",", ":")) + "\n" + json.dumps(rows, separators=(",", ":")),
                 "Bounded tail 2", "CACHE_READY", phase="failure_reproduction")
        # Fresh, early-prefix nonces give a within-sample counter contrast and
        # disjoint response markers; they do not prove provider-wide coldness.
        nonce = uuid.uuid4().hex
        for scope in ("project_A_QA", "project_B_QA", "project_A_developer"):
            marker = "SCOPE_" + scope + "_" + nonce
            prefix = nonce + "/" + scope + ": reply exactly " + marker + ".\n" + json.dumps(rows, separators=(",", ":"))
            for repeat in (1, 2):
                call(scope + "/" + str(repeat), prefix, "Return this scope's marker only.", marker, phase="fresh_scope")
        status, balance = metadata("/balance")
        report["balanceAfter"] = {"httpStatus": status, "body": balance}
        report["completedAt"] = datetime.datetime.now(datetime.timezone.utc).isoformat()
        report["markerPasses"] = sum(row["markerPassed"] for row in report["cases"])
        report["passed"] = len(report["cases"]) == 24 and report["markerPasses"] == 24
    except Exception as error:
        report["passed"] = False
        report["error"] = str(error).replace(key, "[redacted]")[:300]
        raise
    finally:
        save()
    return 0 if report["passed"] else 2


if __name__ == "__main__":
    raise SystemExit(run())
