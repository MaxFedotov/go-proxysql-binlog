#!/bin/bash

# Simple packaging of proxysql-binlog
#
# Requires fpm: https://github.com/jordansissel/fpm
#
# Based on https://github.com/github/orchestrator/blob/master/build.sh
#
set -e

basedir=$(cd -P -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
GIT_COMMIT=$(git rev-parse HEAD)
RELEASE_VERSION=
export RELEASE_VERSION release_base_path
export GO15VENDOREXPERIMENT=1
export CGO_ENABLED=0

binary_build_path="build"
binary_artifact="$binary_build_path/bin/proxysql-binlog"
release_base_path="$basedir/$binary_build_path/release"
skip_build=""
build_only=""
build_paths=""
retain_build_paths=""
opt_race=

usage() {
  echo
  echo "Usage: $0 [-t target ] [-h] [-d] [-r]"
  echo "Options:"
  echo "-h Show this screen"
  echo "-t (linux|darwin) Target OS Default:(linux)"
  echo "-d debug output"
  echo "-b build only, do not generate packages"
  echo "-N do not build; use existing ./buld/bin/proxysql-binlog binary"
  echo "-P create build/deployment paths"
  echo "-R retain existing build/deployment paths"
  echo "-r build with race detector"
  echo "-v release version (optional; default: content of RELEASE_VERSION file)"
  echo
}


function fail() {
  local message="${1}"

  export message
  (>&2 echo "$message")
  exit 1
}

function debug() {
  local message="${1}"

  export message
  (>&2 echo "[DEBUG] $message")
}


function precheck() {
  local target="$1"
  local build_only="$2"
  local ok=0 # return err. so shell exit code

  if [[ "$target" == "linux" ]]; then
    if [[ -z "$build_only" ]] && [[ ! -x "$( which fpm )" ]]; then
      echo "Please install fpm and ensure it is in PATH (typically: 'gem install fpm')"
      ok=1
    fi

    if [[ -z "$build_only" ]] && [[ ! -x "$( which rpmbuild )" ]]; then
      echo "rpmbuild not in PATH, rpm will not be built (OS/X: 'brew install rpm')"
    fi
  fi

  if [[ -z "$GOPATH" ]]; then
    echo "GOPATH not set"
    ok=1
  fi

  if [[ ! -x "$( which go )" ]]; then
    echo "go binary not found in PATH"
    ok=1
  fi

  if ! go version | egrep -q 'go(1\.1[234])' ; then
    echo "go version must be 1.12 or above"
    ok=1
  fi

  return $ok
}

setup_build_path() {
  local build_path

  mkdir -p $release_base_path
  if [ -z "$retain_build_paths" ] ; then
    rm -rf ${release_base_path:?}/*
  fi
  build_path=$(mktemp -d $release_base_path/proxysql-binlogXXXXXX) || fail "Unable to 'mktemp -d $release_base_path/proxysql-binlogXXXXXX'"

  echo $build_path
}

build_binary() {
  local target gobuild
  os="$1"
  ldflags="-X main.Version=${RELEASE_VERSION} -X main.GitCommit=${GIT_COMMIT}"
  debug "Building via $(go version)"
  mkdir -p "$binary_build_path/bin"
  rm -f $binary_artifact
  gobuild="go build -i ${opt_race} -ldflags \"$ldflags\" -o $binary_artifact"

  case $os in
    'linux')
      echo "GOOS=$os GOARCH=amd64 $gobuild" | bash
    ;;
    'darwin')
      echo "GOOS=darwin GOARCH=amd64 $gobuild" | bash
    ;;
  esac
  find $binary_artifact -type f || fail "Failed to generate proxysql-binlog binary"
}

copy_binary_artifacts() {
  build_path="$1"
  cp $binary_artifact "$build_path/proxysql-binlog/usr/bin/" && debug "binary copied to $build_path/proxysql-binlog/usr/bin/" || fail "Failed to copy proxysql-binlog binary to $build_path/proxysql-binlog/usr/bin/"
}

setup_artifact_paths() {
  local build_path
  build_path="$1"

  mkdir -p $build_path/proxysql-binlog
  mkdir -p $build_path/proxysql-binlog/usr/bin/
  mkdir -p $build_path/proxysql-binlog/etc/systemd/system
  mkdir -p $build_path/proxysql-binlog/var/log/proxysql-binlog
  ln -s $build_path $release_base_path/build
}

function copy_resource_artifacts() {
  local build_path
  build_path="$1"

  cd  $basedir
  gofmt -s -w  .
  rsync -qa ./conf/proxysql-binlog.cnf $build_path/proxysql-binlog/etc/
  cp etc/systemd/proxysql-binlog.service $build_path/proxysql-binlog/etc/systemd/system/proxysql-binlog.service
  rsync -qa ./build/scripts/pre-install $build_path/tmp/
}

package_linux() {
  local build_path
  build_path="$1"

  local do_tar=1
  local do_rpm=1
  local do_deb=1

  cd $basedir

  tmp_build_path="$release_base_path/build/tmp"
  mkdir -p $tmp_build_path
  rm -f "${tmp_build_path:-?}/*.*"


  cd $tmp_build_path

  debug "Creating Linux Tar package"
  [ $do_tar -eq 1 ] && tar -C $build_path/proxysql-binlog -czf $release_base_path/proxysql-binlog-"${RELEASE_VERSION}"-$target-amd64.tar.gz ./

  debug "Creating Distro full packages"
  [ $do_rpm -eq 1 ] && fpm -v "${RELEASE_VERSION}" --epoch 1 -f -s dir -n proxysql-binlog -m max-fedotov --description "Proxysql-binlog: service for sending GTID info to ProxySQL" --url "https://github.com/MaxFedotov/go-proxysql-binlog" --vendor "Max Fedotov" --license "Apache 2.0" -C $build_path/proxysql-binlog --prefix=/ --config-files /etc/proxysql-binlog.cnf --rpm-os linux --before-install $build_path/tmp/pre-install --rpm-attr 744,proxysql_binlog,proxysql_binlog:/var/log/proxysql-binlog -t rpm .
  [ $do_deb -eq 1 ] && fpm -v "${RELEASE_VERSION}" --epoch 1 -f -s dir -n proxysql-binlog -m max-fedotov --description "Proxysql-binlog: service for sending GTID info to ProxySQL" --url "https://github.com/MaxFedotov/go-proxysql-binlog" --vendor "Max Fedotov" --license "Apache 2.0" -C $build_path/proxysql-binlog --prefix=/ --config-files /etc/proxysql-binlog.cnf --before-install $build_path/tmp/pre-install -t deb --deb-no-default-config-files .


  debug "packages:"
  for f in * ; do debug "- $f" ; done

  mv ./*.* $release_base_path/
  cd $basedir
}

package_darwin() {
  local build_path
  build_path="$1"

  cd $release_base_path
  debug "Creating Darwin full Package"
  tar -C $build_path/proxysql-binlog -czf $release_base_path/proxysql-binlog-"${RELEASE_VERSION}"-$target-amd64.tar.gz ./
}

package() {
  local target build_path
  target="$1"
  build_path="$2"

  debug "Release version is ${RELEASE_VERSION} (${GIT_COMMIT})"

  case $target in
    'linux') package_linux "$build_path" ;;
    'darwin') package_darwin "$build_path" ;;
  esac
  
  debug "Done. Find releases in $release_base_path"
}

main() {
  local target="$1"
  local build_only=$2
  local build_path

  if [ -z "${RELEASE_VERSION}" ] ; then
    RELEASE_VERSION=$(cat $basedir/VERSION)
  fi

  precheck "$target" "$build_only"
  if [ -z "$skip_build" ] ; then
    build_binary "$target" || fail "Failed building binary"
  fi
  if [ "$build_paths" == "true" ] ; then
    build_path=$(setup_build_path)
    setup_artifact_paths "$build_path"
    copy_resource_artifacts "$build_path"
    copy_binary_artifacts "$build_path"
  fi
  if [[ -z "$build_only" ]]; then
    package "$target" "$build_path"
  fi
}

while getopts "t:v:dbNPRhr" flag; do
  case $flag in
  t)
    target="${OPTARG}"
    ;;
  h)
    usage
    exit 0
    ;;
  d)
    debug=1
    ;;
  b)
    debug "Build only; no packaging"
    build_only="true"
    ;;
  N)
    debug "skipping build"
    [ -f "$binary_artifact" ] || fail "cannot find $binary_artifact"
    skip_build="true"
    ;;
  P)
    debug "Creating build paths"
    build_paths="true"
    ;;
  R)
    debug "Retaining existing build paths"
    retain_build_paths="true"
    ;;
  r)
    opt_race="-race"
    ;;
  v)
    RELEASE_VERSION="${OPTARG}"
    ;;
  ?)
    usage
    exit 2
    ;;
  esac
done

shift $(( OPTIND - 1 ));

if [ -z "$build_only" ] ; then
  # To build packages means we also need to build the paths
  build_paths="true"
fi

if [ -z "$target" ]; then
	uname=$(uname)
	case $uname in
    Linux)	target=linux ;;
    Darwin)	target=darwin ;;
    *)      fail "Unexpected OS from uname: $uname. Exiting" ;;
	esac
fi

[[ $debug -eq 1 ]] && set -x
main "$target" "$build_only"

debug "proxysql-binlog build done; exit status is $?"
