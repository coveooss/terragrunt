#!/bin/sh
# Generate test coverage statistics for Go packages.
#
# Works around the fact that `go test -coverprofile` currently does not work
# with multiple packages, see https://code.google.com/p/go/issues/detail?id=6909
#
# Usage: script/coverage [--html|--codecov]
#
#     --html     Additionally create HTML report and open it in browser
#     --codecov  Create coverage.txt to be uploaded to codecov.io
#
# Based on https://github.com/mlafeldt/chef-runner/blob/v0.7.0/script/coverage

set -e

workdir=.cover
profile="$workdir/cover.out"
mode=count

generate_cover_data() {
    rm -rf "$workdir"
    mkdir "$workdir"

    for pkg in "$@"; do
        f="$workdir/$(echo $pkg | tr / -).cover"
        go test -covermode="$mode" -short -coverprofile="$f" -coverpkg=./... "$pkg"
    done

    echo "mode: $mode" >"$profile"
    grep -h -v "^mode:" "$workdir"/*.cover >>"$profile"
}

show_cover_report() {
    go tool cover -${1}="$profile"
}

create_file_for_codecov() {
    echo "Creating coverage.txt for codecov"
    cat $profile >> coverage.txt
}

generate_cover_data $(go list ./...)
show_cover_report func
case "$1" in
"")
    ;;
--html)
    show_cover_report html ;;
--codecov)
    create_file_for_codecov ;;
*)
    echo >&2 "error: invalid option: $1"; exit 1 ;;
esac

rm "$workdir"/*.cover
rm "$profile"