---
name: elasticsearch-query
description: Elasticsearch operations: search, count, mapping, indices, cluster health, aliases. Supports basic auth and API key.
activation_keywords: [elasticsearch, es, elastic, search, index, mapping, cluster, lucene]
execution_mode: server
---

# Elasticsearch Query Skill

Provides Elasticsearch operations for querying and managing indices:
- **search**: Execute search queries with ES query DSL
- **count**: Get document count for index
- **mapping**: Get index mapping definition
- **indices**: List all indices
- **cluster_health**: Get cluster health status
- **aliases**: List all aliases

Use `builtin_es_query` tool with fields:
- `operation`: Operation name (search, count, mapping, indices, cluster_health, aliases)
- `addresses`: ES addresses (comma-separated, e.g., localhost:9200)
- `index`: Index name to query
- `username`: (optional) ES username
- `password`: (optional) ES password
- `api_key`: (optional) ES API key (alternative to username/password)
- `query`: ES query JSON (for search/count)
- `size`: Number of results (default: 10)
- `_source`: Fields to return (comma-separated)
- `timeout`: Request timeout in seconds (default: 30)

Example - search:
```
operation: "search"
addresses: "localhost:9200"
index: "my-logs"
query: "{\"query\": {\"match\": {\"level\": \"error\"}}}"
size: "20"
```

Example - indices:
```
operation: "indices"
addresses: "localhost:9200"
```

Example - cluster health:
```
operation: "cluster_health"
addresses: "es-node1:9200,es-node2:9200"
```