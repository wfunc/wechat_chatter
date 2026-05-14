#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ONEBOT_DIR="$ROOT_DIR/onebot"
RELEASE_DIR="$ROOT_DIR/release"
BINARY_NAME="${BINARY_NAME:-onebot}"
FRIDA_VERSION="${FRIDA_VERSION:-17.9.8}"

GOOS_VALUE="$(go env GOOS)"
GOARCH_VALUE="$(go env GOARCH)"

case "${GOOS_VALUE}-${GOARCH_VALUE}" in
  darwin-arm64)
    FRIDA_PLATFORM="macos-arm64"
    ARCHIVE_EXT="tar.xz"
    ;;
  darwin-amd64)
    FRIDA_PLATFORM="macos-x86_64"
    ARCHIVE_EXT="tar.xz"
    ;;
  linux-amd64)
    FRIDA_PLATFORM="linux-x86_64"
    ARCHIVE_EXT="tar.xz"
    ;;
  linux-arm64)
    FRIDA_PLATFORM="linux-arm64"
    ARCHIVE_EXT="tar.xz"
    ;;
  *)
    echo "不支持自动下载 Frida devkit: ${GOOS_VALUE}-${GOARCH_VALUE}" >&2
    echo "请手动设置 FRIDA_DEVKIT=/path/to/frida-core-devkit 后重试。" >&2
    exit 1
    ;;
esac

FRIDA_DEVKIT="${FRIDA_DEVKIT:-$ROOT_DIR/.deps/frida-core-devkit/${FRIDA_VERSION}-${FRIDA_PLATFORM}}"
FRIDA_ARCHIVE="frida-core-devkit-${FRIDA_VERSION}-${FRIDA_PLATFORM}.${ARCHIVE_EXT}"
FRIDA_URL="https://github.com/frida/frida/releases/download/${FRIDA_VERSION}/${FRIDA_ARCHIVE}"
FRIDA_ARCHIVE_PATH="$ROOT_DIR/.deps/downloads/$FRIDA_ARCHIVE"

if [[ ! -f "$FRIDA_DEVKIT/frida-core.h" || ! -f "$FRIDA_DEVKIT/libfrida-core.a" ]]; then
  echo "准备 Frida devkit: $FRIDA_PLATFORM $FRIDA_VERSION"
  mkdir -p "$(dirname "$FRIDA_ARCHIVE_PATH")" "$FRIDA_DEVKIT"

  if [[ ! -s "$FRIDA_ARCHIVE_PATH" ]]; then
    echo "下载 $FRIDA_URL"
    curl -L --fail --retry 3 -o "$FRIDA_ARCHIVE_PATH" "$FRIDA_URL"
  fi

  rm -rf "$FRIDA_DEVKIT"
  mkdir -p "$FRIDA_DEVKIT"
  tar -xJf "$FRIDA_ARCHIVE_PATH" -C "$FRIDA_DEVKIT"
fi

if [[ ! -f "$FRIDA_DEVKIT/frida-core.h" || ! -f "$FRIDA_DEVKIT/libfrida-core.a" ]]; then
  echo "Frida devkit 缺少 frida-core.h 或 libfrida-core.a: $FRIDA_DEVKIT" >&2
  exit 1
fi

echo "编译 OneBot..."
(
  cd "$ONEBOT_DIR"
  CGO_ENABLED=1 \
    CGO_CFLAGS="-I$FRIDA_DEVKIT ${CGO_CFLAGS:-}" \
    CGO_LDFLAGS="-L$FRIDA_DEVKIT ${CGO_LDFLAGS:-}" \
    go build -trimpath -o "$BINARY_NAME" .
)

mkdir -p "$RELEASE_DIR"
OUTPUT_TAR="$RELEASE_DIR/${BINARY_NAME}_${GOOS_VALUE}_${GOARCH_VALUE}.tar.gz"

echo "打包 $OUTPUT_TAR"
tar -czf "$OUTPUT_TAR" \
  -C "$ONEBOT_DIR" "$BINARY_NAME" script.js \
  -C "$ROOT_DIR" wechat_version

echo "完成:"
echo "  二进制: $ONEBOT_DIR/$BINARY_NAME"
echo "  压缩包: $OUTPUT_TAR"
