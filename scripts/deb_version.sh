#! /bin/sh

CODE_VERSION=$1
CHANGELOG_VERSION=$(dpkg-parsechangelog | sed -n 's/^Version: //p')
DISTRIBUTION=$(dpkg-parsechangelog | sed -n 's/^Distribution: //p')
PRERELEASE=false

if [ "$2" = dev ]; then
    PRERELEASE=true
elif [ "$2" != '' ]; then
    CODE_VERSION=$CODE_VERSION~$2
fi

dpkg --compare-versions $CHANGELOG_VERSION lt $CODE_VERSION 
if [ $? -eq 0 ]; then
    if [ $DISTRIBUTION = UNRELEASED ]; then
        echo Tagging old release as released...
        dch --release --controlmaint "Releasing to channels."
    fi
    DCH_CMD=""
    if [ $PRERELEASE != true ]; then
        DCH_CMD="--distribution unstable"
    fi
    dch $DCH_CMD --newversion $CODE_VERSION --controlmaint "New release! Check CHANGELOG.md or Consul's homepage for changes"
fi
