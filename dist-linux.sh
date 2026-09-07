#!/bin/bash
set -euo pipefail

export GOOS=linux
export GOARCH=amd64
export CGO_ENABLED=1
export CC=gcc
export CXX=g++
export BUILD_DIR=./dist/build-linux
export TARGET_DIR=./dist/artifacts

if [[ $# -ne 1 ]]
then
  echo "usage: $0 VERSION" >&2
  exit 2
fi

version=$1
script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
bash "$script_dir/scripts/validate-semver.sh" "$version"

exec=$version
build=$version
branch=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "")

mkdir -p $BUILD_DIR

go run tools/assets/assets.go ./ $BUILD_DIR/

# Upstream cimgui-go ships a non-PIC Linux archive, but danser-core.so is a
# shared library. Rebuild the archive with position-independent code before
# cgo links it into the shared core.
cimgui_module=$(go list -m -f '{{.Dir}}' github.com/AllenDang/cimgui-go)
cimgui_build_dir=$BUILD_DIR/cimgui

cmake -S "$cimgui_module/lib" -B "$cimgui_build_dir" \
  -DCMAKE_BUILD_TYPE=Release \
  -DCMAKE_POSITION_INDEPENDENT_CODE=ON
cmake --build "$cimgui_build_dir" --parallel

chmod u+w "$cimgui_module/lib/linux/x64" "$cimgui_module/lib/linux/x64/cimgui.a"
cp "$cimgui_build_dir/cimgui.a" "$cimgui_module/lib/linux/x64/cimgui.a"

go build -trimpath -ldflags "-s -w -X 'github.com/innovationreadytupperware/danser-ee/build.Version=$build' -X 'github.com/innovationreadytupperware/danser-ee/build.Stream=Release' -X 'github.com/innovationreadytupperware/danser-ee/build.Branch=$branch'" -buildmode=c-shared -o $BUILD_DIR/danser-core.so -v -x -tags "exclude_cimgui_glfw exclude_cimgui_sdli"

mv $BUILD_DIR/danser-core.so $BUILD_DIR/libdanser-core.so
cp {libbass.so,libbass_fx.so,libbassmix.so,libyuv.so,libSDL3.so} $BUILD_DIR/

gcc -no-pie --verbose -O3 -o $BUILD_DIR/danser-cli -I. cmain/main_danser.c -I$BUILD_DIR/ -Wl,-rpath,'$ORIGIN' -L$BUILD_DIR/ -ldanser-core

gcc -no-pie --verbose -O3 -D LAUNCHER -o $BUILD_DIR/danser -I. cmain/main_danser.c -I$BUILD_DIR/ -Wl,-rpath,'$ORIGIN' -L$BUILD_DIR/ -ldanser-core

rm $BUILD_DIR/danser-core.h

go run tools/ffmpeg/ffmpeg.go $BUILD_DIR/

# Staged files inherit the build host's modes. We need to normalize what the archive records
chmod 755 "$BUILD_DIR/danser" "$BUILD_DIR/danser-cli" "$BUILD_DIR"/ffmpeg/bin/*
chmod 644 "$BUILD_DIR"/assets.dpak "$BUILD_DIR"/*.so "$BUILD_DIR"/ffmpeg/lib/*

mkdir -p $TARGET_DIR

go run tools/pack2/pack.go $TARGET_DIR/danser-$exec-linux.zip $BUILD_DIR/

rm -rf $BUILD_DIR
