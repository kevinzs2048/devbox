# Multiple Node Development
Lustre Multiple node development needs to have a quick deployment and build test method.
Traditionally, we needs to use Lustre lbuild script to build the kernel and lustre RPM, then copy to the target deployment
nodes, install and reboot(if installed newly built kernel). The Lustre build process is verrry slow since it will do a lot
of dependencies check with ./configure. Usually one building process will cost more than 2 hours, make it verry hard and
slow to do the debug.

So the most easy way to accelerate this process is to avoid the configure process running. Also Lustre offers another way
to running the Lustre setup and test without installing to the OS.

Here we have 1 build machines, and 5 testing nodes. Some preconditions:
- For Kernel: It's better to have the same linux kernel running in Build machine and 5 testing nodes
  - If not, should use lbuild in build machine first to build kernel and Lustre.
  - Then we have the build path for Lustre repo and kernel RPM. Install the RPM kernel to 5 testing nodes and reboot.
- For Lustre: The Lustre code path in all the nodes should be the `same`.
- `All the kmod-lustre* and lustre-* RPM should be removed from the 5 testing nodes.`

## In the build machine
The Lbuild already setup before, with the build path: 
`/root/test/build-0629/lustre/`

So we need to create the dir path in the 5 test node first.

make some code change, and then:
```angular2html
make -j48
```

After that, run the rsync:

```angular2html
#!/bin/bash

cp ~/multinode.sh /root/test/build-0629/lustre/lustre/tests/cfg/
rsync -av --delete -e "ssh -i $HOME/id_rsa" /root/test/build-0629/lustre/* root@213.146.155.114:/root/test/build-0629/lustre/
rsync -av --delete -e "ssh -i $HOME/id_rsa" /root/test/build-0629/lustre/* root@213.146.155.18:/root/test/build-0629/lustre/
rsync -av --delete -e "ssh -i $HOME/id_rsa" /root/test/build-0629/lustre/* root@213.146.155.81:/root/test/build-0629/lustre/
rsync -av --delete -e "ssh -i $HOME/id_rsa" /root/test/build-0629/lustre/* root@213.146.155.12:/root/test/build-0629/lustre/
rsync -av --delete -e "ssh -i $HOME/id_rsa" /root/test/build-0629/lustre/* root@213.146.155.94:/root/test/build-0629/lustre/
```

The multinode.sh:
```angular2html
CLIENTCOUNT=2
RCLIENTS="lustre-3hdazbcp-01 lustre-3hdazbcp-02"

MDSCOUNT=1
mds_HOST="lustre-3hdazbcp-03"
MDSDEV1="/dev/vdb"
mds3_HOST="lustre-3hdazbcp-03"
MDSDEV3="/dev/vdc"
mds2_HOST="lustre-3hdazbcp-04"
MDSDEV2="/dev/vdb"
mds4_HOST="lustre-3hdazbcp-04"
MDSDEV4="/dev/vdc"

OSTCOUNT=7
ost_HOST="lustre-3hdazbcp-05"
OSTDEV1="/dev/vdb"
ost2_HOST="lustre-3hdazbcp-05"
OSTDEV2="/dev/vdc"
ost3_HOST="lustre-3hdazbcp-05"
OSTDEV3="/dev/vdd"
ost4_HOST="lustre-3hdazbcp-05"
OSTDEV4="/dev/vde"
ost5_HOST="lustre-3hdazbcp-05"
OSTDEV5="/dev/vdf"
ost6_HOST="lustre-3hdazbcp-05"
OSTDEV6="/dev/vdg"
ost7_HOST="lustre-3hdazbcp-05"
OSTDEV7="/dev/vdh"

PDSH="/usr/bin/pdsh -S -Rssh -w"
SHARED_DIRECTORY=${SHARED_DIRECTORY:-/opt/testing/shared}
. /root/test/build-0629/lustre/lustre/tests/cfg/ncli.sh
```

## In the testing nodes
Install packages:
```angular2html
dnf install keyutils-libs-devel  libyaml-devel libnl3-devel zlib-devel gcc -y
```

