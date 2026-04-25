---
name: system-monitor
description: Monitor system resources: CPU, memory, disk, network, processes
activation_keywords: [system, cpu, memory, disk, process, monitor, resource, top, uptime]
execution_mode: client
---

# System Monitor Skill

Provides read-only system resource monitoring:
- CPU usage and load average
- Memory usage (total, used, free, swap)
- Disk usage by filesystem
- Top processes by CPU/memory
- System uptime and load

Use `builtin_system_monitor` tool with fields:
- `operation`: one of "cpu", "memory", "disk", "processes", "uptime", "all"
- `limit`: (optional) number of top processes to show (default: 10)

Note: All operations are read-only.
