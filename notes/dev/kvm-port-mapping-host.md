# Map VM port NAT to host port
![img_1.png](../images/libvirt-network-nat.png)

First check the network info:
```angular2html
$ virsh net-list
...
$ virsh net-edit <network-name>
<network>
  <name>default</name>
  <uuid>e478ae74-d717-400b-99f3-b3893b779eca</uuid>
  <forward mode='nat'/>
  <bridge name='virbr0' stp='on' delay='0'/>
...
```

## Assign the static to VM
With port-forwarding, processes running in a virtual machine can be accessed by others. First, we need to assign a static IP address, and then bind host-guest ports for forwarding.
First, you can find out a MAC address assigned to a virtual machine:
Then, modify the libvirt network configuration to assign a static IP address to the MAC address:
```angular2html
$ virsh net-edit <network-name>
<network>
  ...
  <ip address='192.168.122.1' netmask='255.255.255.0'>
    <dhcp>
      <range start='192.168.122.2' end='192.168.122.254'/>
      <host mac='52:54:00:2a:94:f7' name='<vm-name>' ip='192.168.122.10'/> # Add this line; <vm-name> will be assigned a static IP 192.168.122.10.
    </dhcp>
  </ip>
</network>
```
Then, restart DHCP service by:
```angular2html
$ virsh net-destroy default
$ virsh net-start default
```
## Configure the port forwarding
Please make sure that use the steps below, not just modify the iptables manually.
```angular2html
$ git clone https://github.com/saschpe/libvirt-hook-qemu
$ cd libvirt-hook-qemu
$ <edit hooks.json>
$ sudo make install
$ cp hooks.json /etc/libvirt/hooks
## The above one is essential, or the make install will not override the hooks.json.
```

The command above installs hooks Python script and configuration data (hooks.json) into /etc/libvirt/hooks, which will be executed when we start a new VM.

For example, I have a VM named testvm, and I want to bind port 22 in the VM to port 10000 in the host:
```angular2html
$ cat hooks.json
{
    "testvm": {
        "interface": "virbr0",
        "private_ip": "192.168.122.10",
        "port_map": {
            "tcp": [[10000, 22]]
        }
    }
}
$ sudo make install
$ sudo systemctl restart libvirtd
```


# Reference
https://insujang.github.io/2021-05-03/libvirt-vm-network-accessibility/