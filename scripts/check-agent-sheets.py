#!/usr/bin/env python3
"""Opt-in real-provider checks through the standalone agent CLI; stdlib only."""
import argparse
import json
import pathlib
import queue
import subprocess
import threading
import time

PROMPT = "Import everything you can, do not ask me anything"


def local(value):
    if isinstance(value, str):
        return value.rsplit("/", 1)[-1]
    return value


def value_at(sheet, path):
    value = sheet
    for key in path.split("."):
        if isinstance(value, list):
            value = value[int(key)]
        else:
            value = value[key]
    return value


def compare(sheet, expected):
    failures = []
    for path, wanted in expected.get("equals", {}).items():
        try:
            actual = local(value_at(sheet, path))
        except (KeyError, IndexError, TypeError):
            actual = None
        if actual != wanted:
            failures.append({"path": path, "expected": wanted, "actual": actual})
    for path, wanted in expected.get("contains", {}).items():
        try:
            actual = [local(v) for v in value_at(sheet, path) or []]
        except (KeyError, IndexError, TypeError):
            actual = []
        for item in wanted:
            if item not in actual:
                failures.append({"path": path, "expectedMember": item, "actual": actual})
    for path, wanted in expected.get("textContains", {}).items():
        try:
            actual = value_at(sheet, path)
        except (KeyError, IndexError, TypeError):
            actual = None
        text = " ".join(actual) if isinstance(actual, list) else str(actual or "")
        if " ".join(wanted.split()).casefold() not in " ".join(text.split()).casefold():
            failures.append({"path": path, "expectedText": wanted, "actual": actual})
    for item, count in expected.get("inventory", {}).items():
        equipment = sheet.get("Equipment", {})
        stacks = sum((equipment.get(p) or [] for p in ["Equipped", "Backpack", "Loot"]), [])
        actual = sum(s["Count"] for s in stacks if local(s["Item"]) == item)
        if actual != count:
            failures.append({"path": "inventory." + item, "expected": count, "actual": actual})
    for option in sheet.get("CustomOptions") or []:
        if option.get("Selected", True) and option.get("Name", "").casefold() in [name.casefold() for name in expected.get("nativeNames", [])]:
            failures.append({"path": "CustomOptions", "unexpected": option.get("Name")})
    custom_names = [option.get("Name", "").casefold() for option in sheet.get("CustomOptions") or []]
    for name in expected.get("customNames", []):
        if name.casefold() not in custom_names:
            failures.append({"path": "CustomOptions", "missing": name})
    spells = sheet.get("Spells", {})
    held = [local(v) for key in ["Cantrips", "Known", "Prepared"] for v in spells.get(key) or []]
    for spell in expected.get("spells", []):
        if spell not in held:
            failures.append({"path": "Spells", "missing": spell})
    return failures


def run_case(args, case):
    if pathlib.Path(case["id"]).name != case["id"] or case["id"] in (".", ".."):
        raise ValueError("case ID must be a simple directory name")
    directory = args.out / case["id"]
    directory.mkdir(parents=True, exist_ok=False)
    command = [str(args.cli), "agent", "-pack", str(args.pack), "-config", str(args.config), "-timeout", "5m"]
    (directory / "command.json").write_text(json.dumps({"command": command, "input": {"action": "start", "text": PROMPT, "files": case["files"]}}, indent=2))
    messages = queue.Queue()
    with (directory / "stderr.jsonl").open("w") as errors, (directory / "transcript.jsonl").open("w") as transcript:
        start = time.monotonic()
        process = subprocess.Popen(command, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=errors, text=True)

        def read():
            for line in process.stdout:
                try:
                    messages.put(json.loads(line))
                except ValueError:
                    messages.put({"kind": "error", "error": "Invalid CLI JSON: " + line})
            messages.put({"kind": "exit"})

        reader = threading.Thread(target=read, daemon=True)
        reader.start()

        def send(value):
            process.stdin.write(json.dumps(value) + "\n")
            process.stdin.flush()

        send({"action": "start", "files": case["files"], "text": PROMPT})
        status, reason, revision, resumes = "starting", None, 0, 0
        model_ms, model_calls, tool_calls = [], 0, 0
        provider_failed = False
        result, pending_inspection, stopped_at = {}, False, None
        try:
            while True:
                elapsed = time.monotonic() - start
                if reason is None and elapsed >= 300:
                    reason = "timeout"
                    if status in ("queued", "running"):
                        send({"action": "stop", "revision": revision})
                    send({"action": "inspect"})
                    pending_inspection = True
                    stopped_at = time.monotonic()
                if stopped_at and time.monotonic() - stopped_at > 15:
                    break
                try:
                    message = messages.get(timeout=0.2)
                except queue.Empty:
                    continue
                transcript.write(json.dumps({"elapsedSeconds": round(elapsed, 3), **message}) + "\n")
                transcript.flush()
                kind = message["kind"]
                if kind == "model":
                    provider_failed = provider_failed or message.get("failed", False)
                    model_calls += 1
                    model_ms.append(message["durationMs"])
                    tool_calls += len(message.get("calls") or [])
                elif kind == "result":
                    result = message
                    revision = message["session"]["revision"]
                    status = message["session"]["status"]
                    if pending_inspection and message["action"] == "inspect":
                        break
                elif kind == "status":
                    status, revision = message["status"], message["revision"]
                    if reason is None:
                        if status == "paused" and resumes == 0:
                            resumes += 1
                            send({"action": "resume", "revision": revision})
                        elif status in ("review", "waiting", "failed", "paused"):
                            reason = {"waiting": "asked_question", "failed": "provider_failure" if provider_failed else "import_failure", "paused": "turn_budget"}.get(status, "review")
                            send({"action": "inspect"})
                            pending_inspection = True
                            stopped_at = time.monotonic()
                elif kind == "timeout":
                    if reason is None:
                        reason = "timeout"
                        send({"action": "inspect"})
                        pending_inspection = True
                        stopped_at = time.monotonic()
                elif kind in ("error", "exit"):
                    reason = reason or "cli_failure"
                    break
        finally:
            process.stdin.close()
            try:
                process.wait(timeout=10)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait()
            reader.join(timeout=1)
            process.stdout.close()
        elapsed = time.monotonic() - start
    (directory / "result.json").write_text(json.dumps(result, indent=2, ensure_ascii=False))
    if reason == "asked_question" and not any(event["kind"] == "question" for event in result.get("session", {}).get("events", [])):
        reason = "waiting_without_question"
    failures = compare(result.get("sheet", {}), case["expected"])
    summary = {"id": case["id"], "reason": reason, "status": status, "durationSeconds": round(elapsed, 2), "slow": elapsed > 120, "modelCalls": model_calls, "modelDurationMs": model_ms, "nonProviderSeconds": round(max(0, elapsed - sum(model_ms) / 1000), 3), "toolCalls": tool_calls, "resumes": resumes, "failures": failures, "passed": reason == "review" and not failures, "source": case.get("source"), "pages": case.get("pages")}
    (directory / "summary.json").write_text(json.dumps(summary, indent=2, ensure_ascii=False))
    print(f'{case["id"]}: {reason}, {elapsed:.1f}s, {model_calls} model calls, {len(failures)} mismatches', flush=True)
    return summary


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--cli", type=pathlib.Path, required=True)
    parser.add_argument("--pack", type=pathlib.Path, required=True)
    parser.add_argument("--config", type=pathlib.Path, default=pathlib.Path("config.local.yaml"))
    parser.add_argument("--manifest", type=pathlib.Path, required=True)
    parser.add_argument("--out", type=pathlib.Path, required=True)
    parser.add_argument("--case", action="append", default=[])
    args = parser.parse_args()
    args.out.mkdir(parents=True, exist_ok=True)
    cases = json.loads(args.manifest.read_text())
    selected = [case for case in cases if not args.case or case["id"] in args.case]
    if not selected:
        parser.error("no matching cases")
    summaries = []
    for case in selected:
        summaries.append(run_case(args, case))
        (args.out / "summary.json").write_text(json.dumps(summaries, indent=2))
    if any(not summary["passed"] for summary in summaries):
        raise SystemExit(1)


if __name__ == "__main__":
    main()
