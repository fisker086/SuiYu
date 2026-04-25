---
name: huawei-network-device
description: Huawei network device (switch/router) operations: display vlan, interface, version, ip route, ospf, bgp, rip, bfd, nat, acl, stp, power, fan, temperature, cpu, memory. Uses SSH.
activation_keywords: [huawei, cloudengine, ce, ar, router, switch, display, vlan, interface, ospf, bgp]
execution_mode: client
---

# Huawei Network Device Skill

Provides Huawei network device (switch/router) operations via SSH.

## Switching Operations
- **VLAN**: display vlan
- **Interface**: display interface brief
- **MAC**: display mac-address
- **STP**: display stp brief

## Routing Operations
- **IP Route**: display ip routing-table
- **IP Route Verbose**: display ip routing-table verbose
- **FIB**: display fib
- **Routing Statistics**: display routing-table statistics

## Protocol Operations
- **OSPF Neighbor**: display ospf peer brief
- **OSPF Interface**: display ospf interface
- **OSPF LSDB**: display ospf lsdb
- **BGP Peer**: display bgp peer
- **BGP Routing Table**: display bgp routing-table
- **RIP Neighbor**: display rip neighbor
- **RIP Route**: display rip route

## BFD
- **BFD Session**: display bfd session all

## NAT & ACL
- **NAT Session**: display nat session
- **NAT Server**: display nat server
- **ACL**: display acl all

## System Operations
- **Version**: display version
- **Device**: display device
- **Power**: display power
- **Fan**: display fan
- **Temperature**: display temperature all
- **CPU**: display cpu-usage
- **Memory**: display memory
- **Clock**: display clock
- **License**: display license
- **PoE**: display poe interface
- **QoS**: display qos policy brief

Use `builtin_huawei_switch` tool with fields:
- `operation`: Operation name
- `host`: Device IP address or hostname
- `port`: SSH port (default: 22)
- `user`: SSH username (default: admin)
- `password`: SSH password
- `command`: Custom display command (optional)

Example:
```
operation: "display_ospf_neighbor"
host: "192.168.1.1"
user: "admin"
password: "Huawei123"
```

Example - check routing table:
```
operation: "display_ip_route"
host: "192.168.1.1"
user: "admin"
password: "Huawei123"
```

Example - check NAT sessions:
```
operation: "display_nat_session"
host: "192.168.1.1"
user: "admin"
password: "Huawei123"
```

Note: Requires SSH access to the Huawei device. Commands are read-only (display only).