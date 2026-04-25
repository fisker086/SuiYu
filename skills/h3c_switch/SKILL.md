---
name: h3c-network-device
description: H3C (HP Enterprise) network device (switch/router) operations: display vlan, interface, version, ip route, ospf, bgp, rip, bfd, nat, acl, stp, power, fan, temperature, cpu, memory, link-aggregation. Uses SSH.
activation_keywords: [h3c, hp, comware, router, switch, display, vlan, interface, ospf, bgp]
execution_mode: client
---

# H3C Network Device Skill

Provides H3C (HP Enterprise / Comware) network device (switch/router) operations via SSH.

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
- **OSPF Neighbor**: display ospf peer
- **OSPF Interface**: display ospf interface
- **OSPF LSDB**: display ospf lsdb
- **BGP Peer**: display bgp peer
- **BGP Routing Table**: display bgp routing-table
- **RIP Neighbor**: display rip neighbor
- **RIP Route**: display rip route

## BFD
- **BFD Session**: display bfd session

## NAT & ACL
- **NAT Session**: display nat session
- **NAT Server**: display nat server
- **ACL**: display acl all

## System Operations
- **Version**: display version
- **Device**: display device
- **Power**: display power
- **Fan**: display fan
- **Temperature**: display temperature
- **CPU**: display cpu
- **Memory**: display memory
- **Clock**: display clock
- **QoS**: display qos policy
- **Link Aggregation**: display link-aggregation verbose
- **Port Isolate**: display port-isolate group

Use `builtin_h3c_switch` tool with fields:
- `operation`: Operation name
- `host`: Device IP address or hostname
- `port`: SSH port (default: 22)
- `user`: SSH username (default: admin)
- `password`: SSH password
- `command`: Custom display command (optional)

Example:
```
operation: "display_ospf_neighbor"
host: "192.168.1.2"
user: "admin"
password: "H3CPass123"
```

Example - check routing table:
```
operation: "display_ip_route"
host: "192.168.1.2"
user: "admin"
password: "H3CPass123"
```

Example - check BGP:
```
operation: "display_bgp_peer"
host: "192.168.1.2"
user: "admin"
password: "H3CPass123"
```

Note: Requires SSH access to the H3C device. Commands are read-only (display only).