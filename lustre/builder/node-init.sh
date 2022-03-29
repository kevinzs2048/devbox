#!/bin/bash
set -e

sed -i 's/^.*StrictHostKeyChecking.*$/StrictHostKeyChecking no/g' /etc/ssh/ssh_config

NODE_CONF=/home/centos/workspace/node/lustre-test-node.conf
declare -i x=1
while read line
do
    if [[ $x -eq  1 ]]; then
        CLIENT_NODE1 = $line
    elif [[ $x -eq  2 ]]; then
        CLIENT_NODE2 = $line
    elif [[ $x -eq  3 ]]; then
        MDS_NODE1 = $line
    elif [[ $x -eq  4 ]]; then
        MDS_NODE2 = $line
    elif [[ $x -eq  5 ]]; then
        OST_NODE1 = $line
    else
        echo "Wrong Test Node Numbers!"
    fi
    x+=1
done < $NODE_CONF

function init_test_executor {
    NODE_IP=$1

}
