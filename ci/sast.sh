set -euo pipefail
mkdir -p reports
python3 -m unittest discover -s ci -p 'test_sast_policy.py' -v
semgrep scan --config ci/semgrep-go.json --config ci/semgrep-policy.yaml --strict --metrics=off --disable-version-check --json --output reports/semgrep.json cmd internal
python3 ci/check_sast.py reports/semgrep.json | tee reports/sast-summary.txt
control=$(mktemp -d)
trap 'rm -rf "$control"' EXIT
printf 'package control\nimport "os/exec"\nfunc run(input string) { exec.Command("sh", "-c", input) }\n' > "$control/control.go"
semgrep scan --config ci/semgrep-policy.yaml --no-git-ignore --strict --metrics=off --disable-version-check --json --output reports/sast-control.json "$control/control.go"
set +e
python3 ci/check_sast.py reports/sast-control.json > reports/sast-control.txt
status=$?
set -e
cat reports/sast-control.txt
if test "$status" -ne 1; then
  echo 'The SAST control example was not blocked' >&2
  exit 1
fi
echo 'SAST control: blocking confirmed'
