#!/usr/bin/env bash
# Build chatbang-pro locally. Run from anywhere: ./build.sh
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT"

OUTPUT="${OUTPUT:-chatbang-pro}"
INSTALL_DIR="${INSTALL_DIR:-}"
USER_BIN="${USER_BIN:-${HOME}/.local/bin}"

# Prefer user-local Go, then standard install paths, then PATH.
for candidate in \
	"${HOME}/.local/go/bin" \
	"/usr/local/go/bin" \
	"/usr/lib/go/bin"; do
	if [[ -x "${candidate}/go" ]]; then
		export PATH="${candidate}:${PATH}"
		break
	fi
done

if ! command -v go >/dev/null 2>&1; then
	echo "error: go not found. Install from https://go.dev/dl/ or set PATH to your Go bin." >&2
	exit 1
fi

VERSION="dev"
if command -v git >/dev/null 2>&1 && git -C "$ROOT" rev-parse --git-dir >/dev/null 2>&1; then
	VERSION="$(git -C "$ROOT" describe --tags --always --dirty 2>/dev/null || git -C "$ROOT" rev-parse --short HEAD)"
fi

LDFLAGS="-s -w -X main.version=${VERSION}"
BUILD_PKG="./cmd/chatbang-pro"

echo "==> go version: $(go version)"
echo "==> module:     $(awk '/^module /{print $2}' go.mod)"
echo "==> version:    ${VERSION}"
echo "==> output:     ${ROOT}/${OUTPUT}"

go mod download
go build -ldflags "${LDFLAGS}" -o "${OUTPUT}" "${BUILD_PKG}"
chmod +x "${OUTPUT}"
go vet ./...

echo "==> done:       ${ROOT}/${OUTPUT}"

if [[ -n "${INSTALL_DIR}" ]]; then
	mkdir -p "${INSTALL_DIR}"
	install -m 755 "${OUTPUT}" "${INSTALL_DIR}/${OUTPUT}"
	echo "==> installed:  ${INSTALL_DIR}/${OUTPUT}"
fi

if [[ "${SKIP_INSTALL:-}" != "1" ]]; then
	mkdir -p "${USER_BIN}"
	install -m 755 "${OUTPUT}" "${USER_BIN}/${OUTPUT}"
	echo "==> installed:  ${USER_BIN}/${OUTPUT}"
fi

if [[ "${SYSTEM_INSTALL:-}" == "1" ]]; then
	echo "==> installing to /usr/bin/chatbang-pro (sudo)…"
	sudo cp -f "${OUTPUT}" /usr/bin/chatbang-pro
	echo "==> installed:  /usr/bin/chatbang-pro"
fi
