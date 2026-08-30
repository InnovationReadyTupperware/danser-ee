#!/bin/bash
set -euo pipefail

export GOOS=windows
export GOARCH=amd64
export CGO_ENABLED=1
export CC=x86_64-w64-mingw32-gcc
export CXX=x86_64-w64-mingw32-g++
export CGO_LDFLAGS="-static-libstdc++ -static-libgcc -Wl,-Bstatic -lstdc++ -lpthread -Wl,-Bdynamic"
export WINDRESFLAGS="-F pe-x86-64"
export BUILD_DIR=./dist/build-win
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
version_core=${version%%[-+]*}
IFS=. read -r major minor patch <<< "$version_core"

if ! (( 10#$major <= 65535 && 10#$minor <= 65535 && 10#$patch <= 65535 ))
then
  echo "Windows FILEVERSION components must be no greater than 65535: $version" >&2
  exit 1
fi

ver="$major,$minor,$patch,0"

buildMode=${DANSER_BUILD_MODE:-"0"}

if [[ $buildMode == "0" || $buildMode == "1" ]]
then

preRC='#include "winuser.h"
          1 VERSIONINFO
          FILEVERSION '$ver'
          FILEFLAGSMASK 0x3fL
          FILEOS 0x40004L
          FILETYPE 0x1L
          FILESUBTYPE 0x0L
          BEGIN
            BLOCK "StringFileInfo"
            BEGIN
              BLOCK "040904b0"
              BEGIN
                VALUE "CompanyName", "Wieku"
                VALUE "FileDescription", "danser'

postRC='"
      VALUE "LegalCopyright", "Wieku 2018-2024"
      VALUE "ProductName", "danser"
      VALUE "ProductVersion", "'$build'"
    END
    END
    BLOCK "VarFileInfo"
    BEGIN
      VALUE "Translation", 0x409, 1200
    END
END
2 ICON assets/textures/favicon.ico
'

mkdir -p $BUILD_DIR

resgen='windres -l 0 '$WINDRESFLAGS' -o '$BUILD_DIR'/danser.syso'

resCore=$preRC'-core.dll'$postRC
resDanser=$preRC''$postRC
resLauncher=$preRC' launcher'$postRC

$resgen <<< $resCore

go run tools/assets/assets.go ./ $BUILD_DIR/

cp $BUILD_DIR/danser.syso danser.syso

go build -trimpath -ldflags "-s -w -X 'github.com/innovationreadytupperware/danser-ee/build.VERSION=$build' -X 'github.com/innovationreadytupperware/danser-ee/build.Stream=Release'" -buildmode=c-shared -o $BUILD_DIR/danser-core.dll -v -x -tags "exclude_cimgui_glfw exclude_cimgui_sdli"

rm -f danser.syso

$resgen <<< $resDanser

cp {bass.dll,bass_fx.dll,bassmix.dll,libyuv.dll,SDL3.dll} $BUILD_DIR/

$CC <<< --verbose -O3 -o $BUILD_DIR/danser-cli.exe -I. cmain/main_danser.c -I$BUILD_DIR/ -L$BUILD_DIR/ -ldanser-core $BUILD_DIR/danser.syso -municode

$resgen <<< $resLauncher

$CC <<< --verbose -O3 -D LAUNCHER -o $BUILD_DIR/danser.exe -I. cmain/main_danser.c -I$BUILD_DIR/ -L$BUILD_DIR/ -ldanser-core $BUILD_DIR/danser.syso -municode

rm $BUILD_DIR/{danser.syso,danser-core.h}

go run tools/ffmpeg/ffmpeg.go $BUILD_DIR/

fi

if [[ $buildMode == "0" || $buildMode == "2" ]]
then

mkdir -p $TARGET_DIR

go run tools/pack2/pack.go $TARGET_DIR/danser-$exec-win.zip $BUILD_DIR/

rm -rf $BUILD_DIR

fi
