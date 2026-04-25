---
name: dns-lookup
description: DNS query and resolution tools: A, AAAA, MX, TXT, NS, CNAME records
activation_keywords: [dns, lookup, resolve, record, nameserver, domain, mx, ns, cname]
execution_mode: client
---

# DNS Lookup Skill

Provides DNS query operations:
- Query A, AAAA, MX, TXT, NS, CNAME, SOA records
- Specify custom DNS server
- Show full DNS response details

Use `builtin_dns_lookup` tool with fields:
- `domain`: target domain (required)
- `record_type`: DNS record type (default: A)
- `dns_server`: (optional) custom DNS server to query

Note: Requires network access.
