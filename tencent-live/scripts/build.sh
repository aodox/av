#!/bin/bash

set -e

PROJECT_NAME="tencent-live"
OUTPUT_DIR="./bin"

VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(date +"%Y-%m-%d %H:%M:%S")
GO_VERSION=$(go version | awk '{print $3}')

LDFLAGS="-s -w -X 'main.Version=${VERSION}' -X 'main.BuildTime=${BUILD_TIME}' -X 'main.GoVersion=${GO_VERSION}'"

mkdir -p ${OUTPUT_DIR}

echo "Building ${PROJECT_NAME}..."
echo "Version: ${VERSION}"
echo "Build Time: ${BUILD_TIME}"
echo "Go Version: ${GO_VERSION}"

build() {
    local os=$1
    local arch=$2
    local output="${OUTPUT_DIR}/${PROJECT_NAME}"
    
    if [ "$os" == "windows" ]; then
        output="${output}.exe"
    fi
    
    if [ -n "$os" ] && [ -n "$arch" ]; then
        output="${OUTPUT_DIR}/${PROJECT_NAME}-${os}-${arch}"
        if [ "$os" == "windows" ]; then
            output="${output}.exe"
        fi
        echo "Building for ${os}/${arch}..."
        GOOS=$os GOARCH=$arch go build -ldflags "${LDFLAGS}" -o ${output} ./cmd/server
    else
        echo "Building for current platform..."
        go build -ldflags "${LDFLAGS}" -o ${output} ./cmd/server
    fi
    
    echo "Output: ${output}"
}

case "${1:-local}" in
    local)
        build
        ;;
    linux)
        build linux amd64
        ;;
    linux-arm64)
        build linux arm64
        ;;
    darwin)
        build darwin amd64
        ;;
    darwin-arm64)
        build darwin arm64
        ;;
    windows)
        build windows amd64
        ;;
    all)
        build linux amd64
        build linux arm64
        build darwin amd64
        build darwin arm64
        build windows amd64
        ;;
    *)
        echo "Usage: $0 {local|linux|linux-arm64|darwin|darwin-arm64|windows|all}"
        exit 1
        ;;
esac

echo "Build completed!"
