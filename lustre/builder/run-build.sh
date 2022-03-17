#!/bin/bash

GERRITPATH=/root/test/lustre-release
GERRITREPO="http://review.whamcloud.com/fs/lustre-release"
KERNELSRC=/root/kernel-4.18.0-348.2.1.el8_5/linux-4.18.0-348.2.1.el8_lustre_debug_debug.aarch64
SMPJOBS=$(grep processor /proc/cpuinfo  | wc -l)

export PATH=/usr/lib64/ccache:/usr/local/bin:/bin:/usr/bin:/usr/local/sbin:/usr/sbin:/root/.local/bin:/root/bin

touch /tmp/builder-running-build
if [ -e /tmp/builder-stop-signal ] ; then
	rm -rf /tmp/builder-running-build
	while [ -e /tmp/builder-stop-signal ] ; do sleep 10 ; done

	touch /tmp/builder-running-build
fi

REF=$1
echo REF $REF >/tmp/builder_out.txt

cd $GERRITPATH

(git fetch $GERRITREPO $REF && git checkout FETCH_HEAD ) >>/tmp/builder_out.txt 2>&1
RETVAL=$?

if [ $RETVAL -ne 0 ] ; then
	echo git checkout error!
	rm -rf /tmp/builder-running-build
	exit 10
fi

sh autogen.sh >>/tmp/builder_out.txt 2>&1
RETVAL=$?
if [ $RETVAL -ne 0 ] ; then
	echo autogen.sh error!
	tail -20 /tmp/builder_out.txt | sed 's/^/ /'
	rm -rf /tmp/builder-running-build
	exit 11
fi

./configure --with-linux=$KERNELSRC >>/tmp/builder_out.txt 2>&1
RETVAL=$?
if [ $RETVAL -ne 0 ] ; then
	echo configure error!
	tail -20 /tmp/builder_out.txt | sed 's/^/ /'
	rm -rf /tmp/builder-running-build
	exit 12
fi

make -j $SMPJOBS >>/tmp/builder_out.txt 2>&1
RETVAL=$?
if [ $RETVAL -ne 0 ] ; then
	echo build error!
	tail -20 /tmp/builder_out.txt | sed 's/^/ /'
	rm -rf /tmp/builder-running-build
	exit 13
fi

rm -rf /tmp/builder-running-build
