set -euo pipefail
mkdir -p reports
export COMPOSE_PROJECT_NAME=${COMPOSE_PROJECT_NAME:-kfm-ci-local}
export KFM_CI_IMAGE="kfm-ci:${COMPOSE_PROJECT_NAME}"
compose=(docker compose -f compose.ci.yaml)
cleanup() {
  "${compose[@]}" logs --no-color > reports/deploy-services.log 2>&1 || true
  "${compose[@]}" down --volumes --remove-orphans >> reports/deploy-cleanup.log 2>&1
}
trap cleanup EXIT
if [[ ! ${KFM_CI_DB_PASSWORD:-} =~ ^[A-Za-z0-9]{12,72}$ ]]; then
  echo 'Set KFM_CI_DB_PASSWORD to 12-72 letters and digits' >&2
  exit 1
fi
(cd dist && sha256sum -c kfm.sha256)
"${compose[@]}" build app | tee reports/image-build.log
test "$(docker image inspect "$KFM_CI_IMAGE" --format '{{.Config.User}}')" = '10001:10001'
docker image inspect "$KFM_CI_IMAGE" --format 'Image={{.Id}} User={{.Config.User}}' > reports/image.txt
"${compose[@]}" up -d --wait db
printf '%s\n' 'register ci_user' "$KFM_CI_DB_PASSWORD" 'login ci_user' "$KFM_CI_DB_PASSWORD" 'touch check.txt' 'write check.txt 1' 'KFM_CI_SMOKE_OK' 'cat check.txt' 'history' 'exit' |
  "${compose[@]}" run --rm -T app | tee reports/smoke.log
grep -q KFM_CI_SMOKE_OK reports/smoke.log
if grep -q 'ошибка:' reports/smoke.log; then
  echo 'KFM returned an error during the smoke test' >&2
  exit 1
fi
"${compose[@]}" exec -T db psql -U kfm_ci -d kfm_ci -Atc "SELECT count(*) FROM operations o JOIN users u ON u.id=o.user_id WHERE u.username='ci_user' AND o.operation_type IN ('touch','write','cat') AND o.status='success'" > reports/audit-count.txt
test "$(tr -d '\r\n' < reports/audit-count.txt)" = 3
echo 'Deployment: non-root process, file operations and database audit verified'
