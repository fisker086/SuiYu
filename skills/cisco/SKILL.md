---
name: cisco-ios-network-device
description: Cisco IOS network device (switch/router) operations: show vlan, interface, version, ip route, OSPF, BGP, EIGRP, RIP, NAT, ACL, CEF, spanning-tree, power, environment, cpu, memory, inventory, CDP, port-channel, failover. Uses SSH.
activation_keywords: [cisco, ios, catalyst, router, switch, show, vlan, interface, ospf, bgp, eigrp]
execution_mode: client
---

# Cisco IOS Network Device Skill

Provides Cisco IOS/IOS-XE network device (switch/router) operations via SSH.

## Switching Operations
- **VLAN**: show vlan
- **Interface**: show interfaces
- **Interface Status**: show interface status
- **Interface Trunk**: show interface trunk
- **MAC**: show mac address-table
- **Spanning Tree**: show spanning-tree brief
- **Spanning Tree Detail**: show spanning-tree detail

## Routing Operations
- **IP Route**: show ip route
- **IP Route Detailed**: show ip route detail

## Protocol Operations
- **OSPF**: show ip ospf
- **OSPF Neighbor**: show ip ospf neighbor
- **OSPF Interface**: show ip ospf interface
- **OSPF Database**: show ip ospf database

- **BGP**: show ip bgp
- **BGP Neighbor**: show ip bgp neighbors
- **BGP Routing Table**: show ip bgp routing-table

- **EIGRP**: show ip eigrp protocols
- **EIGRP Neighbor**: show ip eigrp neighbors
- **EIGRP Topology**: show ip eigrp topology

- **RIP Database**: show ip rip database

## NAT & ACL
- **NAT Translation**: show ip nat translations
- **NAT Statistics**: show ip nat statistics
- **Access Lists**: show access-lists

## CEF
- **IP CEF**: show ip cef
- **IP CEF Detail**: show ip cef detail

## High Availability
- **Failover**: show failover (ASA)

## System Operations
- **Version**: show version
- **Inventory**: show inventory
- **Power**: show power
- **Environment**: show environment all
- **CPU**: show cpu
- **Memory**: show memory
- **Clock**: show clock
- **License**: show license
- **VTP**: show vtp status
- **Port Channel**: show port-channel summary
- **CDP Neighbors**: show cdp neighbors
- **CDP Neighbors Detail**: show cdp neighbors detail

Use `builtin_cisco_ios` tool with fields:
- `operation`: Operation name
- `host`: Device IP address or hostname
- `port`: SSH port (default: 22)
- `user`: SSH username (default: admin)
- `password`: SSH password
- `command`: Custom show command (optional)

Example:
```
operation: "show_ospf_neighbor"
host: "192.168.1.3"
user: "admin"
password: "Cisco123"
```

Example - check routing table:
```
operation: "show_ip_route"
host: "192.168.1.3"
user: "admin"
password: "Cisco123"
```

Example - check BGP:
```
operation: "show_bgp_neighbor"
host: "192.168.1.3"
user: "admin"
password: "Cisco123"
```

Example - check NAT:
```
operation: "show_nat_translation"
host: "192.168.1.3"
user: "admin"
password: "Cisco123"
```

Note: Requires SSH access to the Cisco device. Commands are read-only (show only).