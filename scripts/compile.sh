#!/usr/bin/env sh
set -e

if [ -z "$NAME" ]; then
  echo "Missing \$NAME!"
  exit 127
fi

if [ -z "$PROJECT" ]; then
  echo "Missing \$PROJECT!"
  exit 127
fi

# Get the git commit information
GIT_COMMIT="$(git rev-parse --short HEAD)"
GIT_DIRTY="$(test -n "$(git status --porcelain)" && echo "+CHANGES" || true)"

# Remove old builds
rm -rf bin/*
rm -rf pkg/*

# Runtime variables
LDFLAGS="-s -w"
LDFLAGS="$LDFLAGS -X ${PROJECT}/version.Name=${NAME}"
LDFLAGS="$LDFLAGS -X ${PROJECT}/version.Version=${VERSION}"
LDFLAGS="$LDFLAGS -X ${PROJECT}/version.GitCommit=${GIT_COMMIT}${GIT_DIRTY}"

# Build!
for GOOS in $XC_OS; do
  for GOARCH in $XC_ARCH; do
    COMBO="${GOOS}/${GOARCH}"
    if test "${XC_EXCLUDE#*$COMBO}" != "${XC_EXCLUDE}"; then
      printf "%s%20s %s\n" "-->" "${GOOS}/${GOARCH}:" "${PROJECT} (excluded)"
      continue
    fi

    EXT=""
    if test "${GOOS}" = "windows"; then
      EXT=".exe"
    fi

    printf "%s%20s %s\n" "-->" "${GOOS}/${GOARCH}:" "${PROJECT}"
    env -i \
      PATH="$PATH" \
      CGO_ENABLED=0 \
      GOARCH="${GOARCH}" \
      GOOS="${GOOS}" \
      GOPATH="$GOPATH" \
      GOROOT="$GOROOT" \
      GOTAGS=${GOTAGS} \
      go build \
      -a \
      -ldflags="$LDFLAGS" \
      -o="pkg/${GOOS}_${GOARCH}/${NAME}${EXT}" \
      .
  done
done

# If we are not in distribution mode, exit now
if [ -z "$DIST" ]; then
  exit 0
fi

echo "--> Compressing..."

apt-get update -qq >/dev/null 2>&1
apt-get install -yqq --force-yes unzip zip >/dev/null 2>&1

mkdir pkg/dist
for PLATFORM in $(find ./pkg -mindepth 1 -maxdepth 1 -type d); do
  OSARCH=$(basename ${PLATFORM})
  if [ "$OSARCH" = "dist" ]; then
    continue
  fi

  EXT=""
  if test -z "${OSARCH##*windows*}"; then
    EXT=".exe"
  fi

  cd $PLATFORM
  tar -czf ../dist/${NAME}_${VERSION}_${OSARCH}.tgz ${NAME}${EXT}
  zip ../dist/${NAME}_${VERSION}_${OSARCH}.zip ${NAME}${EXT}
  cd - >/dev/null 2>&1
done

echo "--> Checksumming..."
cd pkg/dist
shasum -a256 * > "${NAME}_${VERSION}_SHA256SUMS"
cd - >/dev/null 2>&1
