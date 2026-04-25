---
name: network-tools
description: Network diagnostic tools: ping, traceroute, netstat, ss
activation_keywords: [ping, traceroute, network, netstat, connection, port, socket]
execution_mode: client
---

# Network Tools Skill

Provides read-only network diagnostic operations:
- Ping a host to check connectivity and latency
- Traceroute to trace network path
- List network connections (netstat/ss)
- Check listening ports

Use `builtin_network_tools` tool with fields:
- `operation`: one of "ping", "traceroute", "connections", "listening"
- `host`: (optional) target host for ping/traceroute
- `count`: (optional) ping count (default: 4)

Note: All operations are read-only.
