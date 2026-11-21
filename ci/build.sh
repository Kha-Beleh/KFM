set -euo pipefail
mkdir -p dist
go mod download
go mod verify
go vet ./...
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o dist/kfm ./cmd/kfm
(cd dist && sha256sum kfm > kfm.sha256)
go version > dist/build.txt
printf 'GOOS=linux GOARCH=amd64 CGO_ENABLED=0\n' >> dist/build.txt
