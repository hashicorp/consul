
DIRS="cpu disk docker host load mem net process"

GOOS=`uname | tr '[:upper:]' '[:lower:]'`
ARCH=`uname -m`

case $ARCH in
	amd64)
		GOARCH="amd64"
		;;
	x86_64)
		GOARCH="amd64"
		;;
	i386)
		GOARCH="386"
		;;
	i686)
		GOARCH="386"
		;;
	arm)
		GOARCH="arm"
		;;
	*)
		echo "unknown arch: $ARCH"
		exit 1
esac

for DIR in $DIRS
do
	if [ -e ${DIR}/types_${GOOS}.go ]; then
		echo "// +build $GOOS" > ${DIR}/${DIR}_${GOOS}_${GOARCH}.go
		echo "// +build $GOARCH" >> ${DIR}/${DIR}_${GOOS}_${GOARCH}.go
		go tool cgo -godefs ${DIR}/types_${GOOS}.go >> ${DIR}/${DIR}_${GOOS}_${GOARCH}.go
	fi
done


