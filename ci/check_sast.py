import json
import sys


def check(path):
    with open(path, encoding="utf-8") as report:
        data = json.load(report)
    if not isinstance(data.get("results"), list) or data.get("errors"):
        raise ValueError("Semgrep did not finish a complete scan")
    blocked = []
    for result in data["results"]:
        severity = result["extra"]["severity"].upper()
        if severity not in {"INFO", "WARNING", "ERROR", "LOW", "MEDIUM", "HIGH", "CRITICAL"}:
            raise ValueError("Unknown finding severity")
        if severity in {"ERROR", "HIGH", "CRITICAL"}:
            blocked.append(result)
    print(f"Findings: {len(data['results'])}; blocking: {len(blocked)}")
    for result in blocked:
        print(f"{result['path']}:{result['start']['line']} {result['check_id']}")
    return 1 if blocked else 0


if __name__ == "__main__":
    try:
        sys.exit(check(sys.argv[1]))
    except (OSError, ValueError, KeyError, TypeError, IndexError) as error:
        print(f"Invalid SAST report: {error}", file=sys.stderr)
        sys.exit(2)
