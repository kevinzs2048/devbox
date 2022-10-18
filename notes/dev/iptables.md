# IPtables
## iptables 命令
按行显示nat表的Iptables规则：
```angular2html
iptables -nL -v --line-numbers -t nat
```

```angular2html
Chain PREROUTING (policy ACCEPT 0 packets, 0 bytes)
num   pkts bytes target     prot opt in     out     source               destination
1     1340  111K DOCKER     all  --  *      *       0.0.0.0/0            0.0.0.0/0            ADDRTYPE match dst-type LOCAL
2        0     0 DNAT       tcp  --  *      *       0.0.0.0/0            192.168.212.93       tcp dpt:2298 to:192.168.80.98:22

Chain INPUT (policy ACCEPT 0 packets, 0 bytes)
num   pkts bytes target     prot opt in     out     source               destination

Chain OUTPUT (policy ACCEPT 0 packets, 0 bytes)
num   pkts bytes target     prot opt in     out     source               destination
1        1    60 DOCKER     all  --  *      *       0.0.0.0/0           !127.0.0.0/8          ADDRTYPE match dst-type LOCAL

Chain POSTROUTING (policy ACCEPT 0 packets, 0 bytes)
num   pkts bytes target     prot opt in     out     source               destination
1    38414 2444K MASQUERADE  all  --  *      !docker0  172.17.0.0/16        0.0.0.0/0
2     7902  555K LIBVIRT_PRT  all  --  *      *       0.0.0.0/0            0.0.0.0/0
3        0     0 MASQUERADE  tcp  --  *      *       172.17.0.2           172.17.0.2           tcp dpt:32581
4        0     0 MASQUERADE  tcp  --  *      *       172.17.0.2           172.17.0.2           tcp dpt:5555
5        3   180 MASQUERADE  all  --  *      virbr0  0.0.0.0/0            0.0.0.0/0

Chain DOCKER (2 references)
num   pkts bytes target     prot opt in     out     source               destination
1        0     0 RETURN     all  --  docker0 *       0.0.0.0/0            0.0.0.0/0
2        1    44 DNAT       tcp  --  !docker0 *       0.0.0.0/0            0.0.0.0/0            tcp dpt:32581 to:172.17.0.2:32581
3       54  3132 DNAT       tcp  --  !docker0 *       0.0.0.0/0            0.0.0.0/0            tcp dpt:32561 to:172.17.0.2:5555

Chain LIBVIRT_PRT (1 references)
num   pkts bytes target     prot opt in     out     source               destination
1        4   269 RETURN     all  --  *      *       192.168.122.0/24     224.0.0.0/24
2        0     0 RETURN     all  --  *      *       192.168.122.0/24     255.255.255.255
3     3233  198K MASQUERADE  tcp  --  *      *       192.168.122.0/24    !192.168.122.0/24     masq ports: 1024-65535
4     1016 69434 MASQUERADE  udp  --  *      *       192.168.122.0/24    !192.168.122.0/24     masq ports: 1024-65535
5        1    84 MASQUERADE  all  --  *      *       192.168.122.0/24    !192.168.122.0/24
```

查看filter表规则：
```angular2html
 iptables -L -n --line-number
```
不加-t就是默认显示filter表

删除：
```angular2html
iptables -D INPUT 3  //删除input的第3条规则  
iptables -t nat -D POSTROUTING 1  //删除nat表中postrouting的第一条规则  
iptables -F INPUT   //清空 filter表INPUT所有规则  
iptables -F    //清空所有规则  
iptables -t nat -F POSTROUTING   //清空nat表POSTROUTING所有规则  
```
删除操作：删除nat表中PREROUTING chain的第2条
```angular2html
 iptables -t nat -D PREROUTING 2
```