#!/bin/sh

set -e

# Wraps autotools.sh, but each binary crashes if it exhibits undefined behavior. 
# Set the undefined behavior flags. This crashes on all undefined behavior except for
# undefined casting, aka "vptr".
# TODO: fix undefined vptr behavior and turn this option back on.

export CFLAGS="-fsanitize=undefined -fno-sanitize-recover=undefined -O0 -ggdb3 -fno-omit-frame-pointer"
export CXXFLAGS="${CFLAGS}"
export LDFLAGS="-lubsan"
export UBSAN_OPTIONS=print_stacktrace=1

#
# work around https://svn.boost.org/trac10/ticket/11632 if present
#

sed -i 's/, stream_t(rdbuf()) /, stream_t(pbase_type::member.get())/g' /usr/include/boost/format/alt_sstream.hpp

# llvm-symbolizer must be on PATH to get a stack trace on error

CLANG_PATH="$(mktemp -d)"
trap "rm -rf ${CLANG_PATH}" EXIT
ln -s "$(whereis llvm-symbolizer-4.0  | rev | cut -d ' ' -f 1 | rev)" \
  "${CLANG_PATH}/llvm-symbolizer"
export PATH="${CLANG_PATH}:${PATH}"
llvm-symbolizer -version

build/docker/scripts/autotools.sh $*
