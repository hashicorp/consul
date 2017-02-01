#/bin/sh

# see http://www.songmu.jp/riji/entry/2015-01-15-goveralls-multi-package.html

set -e
# cleanup
cleanup() {
  if [ $tmpprof != "" ] && [ -f $tmpprof ]; then
    rm -f $tmpprof
  fi
  exit
}
trap cleanup INT QUIT TERM EXIT

# メインの処理
prof=${1:-".profile.cov"}
echo "mode: count" > $prof
gopath1=$(echo $GOPATH | cut -d: -f1)
for pkg in $(go list ./...); do
  tmpprof=$gopath1/src/$pkg/profile.tmp
  go test -covermode=count -coverprofile=$tmpprof $pkg
  if [ -f $tmpprof ]; then
    cat $tmpprof | tail -n +2 >> $prof
    rm $tmpprof
  fi
done
