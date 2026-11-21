set -euo pipefail
mkdir -p reports
if [[ ! ${KFM_CI_DB_PASSWORD:-} =~ ^[A-Za-z0-9]{12,72}$ ]]; then
  echo 'Set KFM_CI_DB_PASSWORD to 12-72 letters and digits' >&2
  exit 1
fi
export KFM_TEST_DATABASE_URL="postgres://kfm_ci:${KFM_CI_DB_PASSWORD}@${KFM_TEST_DB_HOST:-db}:5432/kfm_ci?sslmode=disable"
go test -count=1 -timeout=180s -json -coverprofile=reports/coverage.out ./... | tee reports/tests.json
if grep -q '"Action":"skip"' reports/tests.json; then
  echo 'Skipped tests are not accepted in the Linux CI run' >&2
  exit 1
fi
