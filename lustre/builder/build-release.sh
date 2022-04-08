#!/bin/bash
set -e

if [ ! -n "$1" ] ;then
echo "Please specified the build args"
exit 1
fi

BUILD_ARGS=$1
BUILD_LINUX=YES
BUILD_ROOT_DIR=/home/jenkins/build_dir
SOURCE_DIR=$BUILD_ROOT_DIR/source
BUILD_DIR=$BUILD_ROOT_DIR/build-$BUILD_ARGS
LIUNX_DIR=$BUILD_ROOT_DIR/kernel-4.18.0-348.2.1.el8_5/linux-4.18.0-348.2.1.el8_lustre_debug_debug.aarch64/

GERRITPATH=$BUILD_ROOT_DIR/lustre-release
GERRITREPO="http://review.whamcloud.com/fs/lustre-release"

# RPM repo for Lustre and e2fsprogs, Lustre repo also include kernel packages
RPM_REPO=/usr/share/nginx/html/repo

echo "Generate the release tar bz"
cd $GERRITPATH
git fetch origin
git reset --hard origin/master

files=$(ls *.tar.gz 2> /dev/null | wc -l);
if [ "$files" != "0" ] ;then
    yes | rm -i *.tar.gz -rf
fi

# Generate the source file
sh autogen.sh
./configure --enable-dist
make dist

CODEBASE=`find . -name "lustre*tar.gz"`
CODEBASE=${CODEBASE: 2}

if [ ! -f "$CODEBASE" ]; then
    echo "$CODEBASE does not exist"
    exit 1
fi

mkdir -p $SOURCE_DIR/$BUILD_ARGS
cp $CODEBASE $SOURCE_DIR/$BUILD_ARGS/
cd $SOURCE_DIR/$BUILD_ARGS
tar zxf $CODEBASE

CODE_DIR=${CODEBASE%.tar.gz}
mv $CODEBASE $CODEBASE.bk

yes | cp $BUILD_ROOT_DIR/kernel-4.18.0-4.18-rhel8.5-aarch64.config-debug $SOURCE_DIR/$BUILD_ARGS/$CODE_DIR/lustre/kernel_patches/kernel_configs/
tar zcf $CODEBASE $CODE_DIR

if [ ! -f "$CODEBASE" ]; then
    echo "$CODEBASE does not exist"
    exit 1
fi

echo "Executing the Build process"

mkdir -p $BUILD_DIR
cd $BUILD_DIR

if [ -z BUILD_LINUX ]; then
    # Build with exist Linux kernel
    $BUILD_ROOT_DIR/lustre-release/contrib/lbuild/lbuild --lustre=$SOURCE_DIR/$BUILD_ARGS/$CODEBASE --extraversion=debug --enable-kernel-debug --target=4.18-rhel8.5 --distro=rhel8.5 --kerneldir=$SOURCE_DIR  --with-linux=$LIUNX_DIR
    # Remove all the Lustre packages
    sudo rm $RPM_REPO/lustre/*.el8.aarch64.rpm
else
    # Build with Linux kernel
    $BUILD_ROOT_DIR/lustre-release/contrib/lbuild/lbuild --lustre=$SOURCE_DIR/$BUILD_ARGS/$CODEBASE --extraversion=debug --enable-kernel-debug --target=4.18-rhel8.5 --distro=rhel8.5 --kerneldir=$SOURCE_DIR
    # Remove all the Lustre packages and Linux packages
    sudo rm $RPM_REPO/lustre/*.rpm
fi

sudo mv $BUILD_DIR/RPMS/aarch64/*.rpm $RPM_REPO/lustre/
