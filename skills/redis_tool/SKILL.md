---
name: redis-tool
description: Redis operations: string, hash, list, set, zset, info, dbsize, ping. Supports password auth and multiple databases.
activation_keywords: [redis, cache, key-value, kv, string, hash, list, set, zset]
execution_mode: server
---

# Redis Tool Skill

Provides comprehensive Redis operations for cache and data management:
- **String**: get, set, del, keys, exists, type, ttl, expire, incr, decr
- **Hash**: hget, hset, hdel, hgetall
- **List**: lpush, rpush, lrange
- **Set**: sadd, smembers
- **ZSet**: zadd, zrange
- **Info**: info, dbsize, ping

Use `builtin_redis_tool` tool with fields:
- `operation`: Operation name (get, set, del, keys, exists, type, ttl, expire, incr, decr, hash_get, hash_set, hash_del, hash_getall, list_push, list_range, set_add, set_members, zset_add, zset_range, info, dbsize, ping)
- `addr`: Redis address (default: localhost:6379)
- `password`: (optional) Redis password
- `db`: (optional) Database number (default: 0)
- `key`: Key for string/hash/list/set/zset operations
- `value`: Value to set
- `field`: Field for hash operations
- `ttl`: TTL in seconds for set/expire
- `pattern`: Pattern for keys operation (default: *)
- `score`: Score for zset operations
- `member`: Member for set/zset operations
- `direction`: List push direction: left/right (default: left)
- `start`: Start index for list_range/zset_range
- `end`: End index for list_range/zset_range
- `with_scores`: Include scores in zset_range (true/1 or false)
- `section`: Info section (e.g., memory, clients)

Example:
```
operation: get
key: "mykey"
addr: "localhost:6379"
```

On the **AgentSphere desktop** app, `builtin_redis_tool` runs locally via Tauri (`run_client_redis_tool`); connection parameters stay on the device unless execution is overridden to server in skill settings.