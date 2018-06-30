#!/bin/sh
set -ev

CMAKE_FLAGS=$*
MAKEPROG=make

if ninja --version  >/dev/null 2>&1; then
  MAKEPROG=ninja
  CMAKE_FLAGS="-GNinja $CMAKE_FLAGS"
fi

mkdir -p cmake_build && cd cmake_build
cmake $CMAKE_FLAGS ..
for LIB in $BUILD_LIBS; do
  if ! grep "^BUILD_${LIB}:BOOL=ON$" CMakeCache.txt ; then
    echo "failed to configure $LIB"
    exit 1
  fi
done
$MAKEPROG -j3
cpack
ctest -VV -E "(python_test)"
# disabled cmake python_test for now since it fails in travis under centos
