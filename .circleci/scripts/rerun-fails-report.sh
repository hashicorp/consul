#!/usr/bin/env bash
#
# Add a comment on the github PR if there were any rerun tests.
#
set -eu -o pipefail

report_filename="${1?report filename is required}"
if [ ! -s "$report_filename" ]; then
    echo "gotestsum rerun report file is empty or missing"
    exit 0
fi

function report {
    echo ":repeat: gotestsum re-ran some tests in $CIRCLE_BUILD_URL"
    echo
    echo '```'
    cat "$report_filename"
    echo '```'
}

report