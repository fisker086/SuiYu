package skills

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
	einoschema "github.com/cloudwego/eino/schema"
	"github.com/redis/go-redis/v9"
)

const toolRedis = "builtin_redis_tool"

var allowedRedisOps = map[string]bool{
	"get":         true,
	"set":         true,
	"del":         true,
	"keys":        true,
	"exists":      true,
	"type":        true,
	"ttl":         true,
	"expire":      true,
	"incr":        true,
	"decr":        true,
	"hash_get":    true,
	"hash_set":    true,
	"hash_del":    true,
	"hash_getall": true,
	"list_push":   true,
	"list_range":  true,
	"set_add":     true,
	"set_members": true,
	"zset_add":    true,
	"zset_range":  true,
	"info":        true,
	"dbsize":      true,
	"ping":        true,
}

func execBuiltinRedisTool(_ context.Context, in map[string]any) (string, error) {
	op := strArg(in, "operation", "op", "action")
	if op == "" {
		return "", fmt.Errorf("missing operation")
	}

	if !allowedRedisOps[op] {
		return "", fmt.Errorf("operation %q not allowed (allowed: %v)", op, allowedRedisOps)
	}

	addr := strArg(in, "addr", "address", "host")
	if addr == "" {
		addr = "localhost:6379"
	}

	password := strArg(in, "password", "pass", "pwd")
	dbStr := strArg(in, "db", "database")
	db := 0
	if dbStr != "" {
		fmt.Sscanf(dbStr, "%d", &db)
	}

	opts := &redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	}

	client := redis.NewClient(opts)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return "", fmt.Errorf("failed to connect to redis: %w", err)
	}

	switch op {
	case "get":
		key := strArg(in, "key")
		if key == "" {
			return "", fmt.Errorf("missing key")
		}
		val, err := client.Get(ctx, key).Result()
		if err == redis.Nil {
			return fmt.Sprintf("Key '%s' not found", key), nil
		}
		if err != nil {
			return "", fmt.Errorf("get failed: %w", err)
		}
		return fmt.Sprintf("GET %s:\n%s", key, val), nil

	case "set":
		key := strArg(in, "key")
		if key == "" {
			return "", fmt.Errorf("missing key")
		}
		value := strArg(in, "value")
		if value == "" {
			return "", fmt.Errorf("missing value")
		}
		ttl := strArg(in, "ttl", "expire_seconds")
		if ttl != "" {
			var sec int64
			if _, err := fmt.Sscanf(ttl, "%d", &sec); err == nil && sec > 0 {
				err = client.Set(ctx, key, value, time.Duration(sec)*time.Second).Err()
				if err != nil {
					return "", fmt.Errorf("set failed: %w", err)
				}
				return fmt.Sprintf("SET %s (TTL: %ds): OK", key, sec), nil
			}
		}
		err := client.Set(ctx, key, value, 0).Err()
		if err != nil {
			return "", fmt.Errorf("set failed: %w", err)
		}
		return fmt.Sprintf("SET %s: OK", key), nil

	case "del":
		key := strArg(in, "key")
		if key == "" {
			return "", fmt.Errorf("missing key")
		}
		n, err := client.Del(ctx, key).Result()
		if err != nil {
			return "", fmt.Errorf("del failed: %w", err)
		}
		return fmt.Sprintf("DEL %s: removed %d key(s)", key, n), nil

	case "keys":
		pattern := strArg(in, "pattern", "match")
		if pattern == "" {
			pattern = "*"
		}
		keys, err := client.Keys(ctx, pattern).Result()
		if err != nil {
			return "", fmt.Errorf("keys failed: %w", err)
		}
		if len(keys) == 0 {
			return fmt.Sprintf("No keys found matching '%s'", pattern), nil
		}
		return fmt.Sprintf("Keys matching '%s' (%d):\n%s", pattern, len(keys), strings.Join(keys, "\n")), nil

	case "exists":
		key := strArg(in, "key")
		if key == "" {
			return "", fmt.Errorf("missing key")
		}
		n, err := client.Exists(ctx, key).Result()
		if err != nil {
			return "", fmt.Errorf("exists failed: %w", err)
		}
		return fmt.Sprintf("Key '%s' exists: %v", key, n == 1), nil

	case "type":
		key := strArg(in, "key")
		if key == "" {
			return "", fmt.Errorf("missing key")
		}
		t, err := client.Type(ctx, key).Result()
		if err != nil {
			return "", fmt.Errorf("type failed: %w", err)
		}
		return fmt.Sprintf("Key '%s' type: %s", key, t), nil

	case "ttl":
		key := strArg(in, "key")
		if key == "" {
			return "", fmt.Errorf("missing key")
		}
		d, err := client.TTL(ctx, key).Result()
		if err != nil {
			return "", fmt.Errorf("ttl failed: %w", err)
		}
		if d < 0 {
			return fmt.Sprintf("Key '%s' has no expiry", key), nil
		}
		return fmt.Sprintf("Key '%s' TTL: %v", key, d), nil

	case "expire":
		key := strArg(in, "key")
		if key == "" {
			return "", fmt.Errorf("missing key")
		}
		ttl := strArg(in, "ttl", "seconds")
		if ttl == "" {
			return "", fmt.Errorf("missing ttl")
		}
		var sec int64
		if _, err := fmt.Sscanf(ttl, "%d", &sec); err != nil {
			return "", fmt.Errorf("invalid ttl: %s", ttl)
		}
		ok, err := client.Expire(ctx, key, time.Duration(sec)*time.Second).Result()
		if err != nil {
			return "", fmt.Errorf("expire failed: %w", err)
		}
		return fmt.Sprintf("EXPIRE %s %ds: %v", key, sec, ok), nil

	case "incr":
		key := strArg(in, "key")
		if key == "" {
			return "", fmt.Errorf("missing key")
		}
		val, err := client.Incr(ctx, key).Result()
		if err != nil {
			return "", fmt.Errorf("incr failed: %w", err)
		}
		return fmt.Sprintf("INCR %s: %d", key, val), nil

	case "decr":
		key := strArg(in, "key")
		if key == "" {
			return "", fmt.Errorf("missing key")
		}
		val, err := client.Decr(ctx, key).Result()
		if err != nil {
			return "", fmt.Errorf("decr failed: %w", err)
		}
		return fmt.Sprintf("DECR %s: %d", key, val), nil

	case "hash_get":
		key := strArg(in, "key")
		if key == "" {
			return "", fmt.Errorf("missing key")
		}
		field := strArg(in, "field")
		if field == "" {
			return "", fmt.Errorf("missing field")
		}
		val, err := client.HGet(ctx, key, field).Result()
		if err == redis.Nil {
			return fmt.Sprintf("Field '%s' not found in hash '%s'", field, key), nil
		}
		if err != nil {
			return "", fmt.Errorf("hash_get failed: %w", err)
		}
		return fmt.Sprintf("HGET %s %s:\n%s", key, field, val), nil

	case "hash_set":
		key := strArg(in, "key")
		if key == "" {
			return "", fmt.Errorf("missing key")
		}
		field := strArg(in, "field")
		if field == "" {
			return "", fmt.Errorf("missing field")
		}
		value := strArg(in, "value")
		if value == "" {
			return "", fmt.Errorf("missing value")
		}
		err := client.HSet(ctx, key, field, value).Err()
		if err != nil {
			return "", fmt.Errorf("hash_set failed: %w", err)
		}
		return fmt.Sprintf("HSET %s %s: OK", key, field), nil

	case "hash_del":
		key := strArg(in, "key")
		if key == "" {
			return "", fmt.Errorf("missing key")
		}
		field := strArg(in, "field")
		if field == "" {
			return "", fmt.Errorf("missing field")
		}
		n, err := client.HDel(ctx, key, field).Result()
		if err != nil {
			return "", fmt.Errorf("hash_del failed: %w", err)
		}
		return fmt.Sprintf("HDEL %s %s: removed %d field(s)", key, field, n), nil

	case "hash_getall":
		key := strArg(in, "key")
		if key == "" {
			return "", fmt.Errorf("missing key")
		}
		m, err := client.HGetAll(ctx, key).Result()
		if err != nil {
			return "", fmt.Errorf("hash_getall failed: %w", err)
		}
		if len(m) == 0 {
			return fmt.Sprintf("Hash '%s' is empty", key), nil
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("HGETALL %s:\n", key))
		for f, v := range m {
			b.WriteString(fmt.Sprintf("  %s: %s\n", f, v))
		}
		return b.String(), nil

	case "list_push":
		key := strArg(in, "key")
		if key == "" {
			return "", fmt.Errorf("missing key")
		}
		value := strArg(in, "value")
		if value == "" {
			return "", fmt.Errorf("missing value")
		}
		dir := strArg(in, "direction", "pos")
		var err error
		var n int64
		if dir == "right" || dir == "rpush" {
			n, err = client.RPush(ctx, key, value).Result()
		} else {
			n, err = client.LPush(ctx, key, value).Result()
		}
		if err != nil {
			return "", fmt.Errorf("list_push failed: %w", err)
		}
		return fmt.Sprintf("LPUSH/RPUSH %s: list length now %d", key, n), nil

	case "list_range":
		key := strArg(in, "key")
		if key == "" {
			return "", fmt.Errorf("missing key")
		}
		start := strArg(in, "start", "offset")
		end := strArg(in, "end", "count")
		s, e := 0, -1
		if start != "" {
			fmt.Sscanf(start, "%d", &s)
		}
		if end != "" {
			fmt.Sscanf(end, "%d", &e)
		}
		vals, err := client.LRange(ctx, key, int64(s), int64(e)).Result()
		if err != nil {
			return "", fmt.Errorf("list_range failed: %w", err)
		}
		if len(vals) == 0 {
			return fmt.Sprintf("List '%s' is empty", key), nil
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("LRANGE %s (%d items):\n", key, len(vals)))
		for i, v := range vals {
			b.WriteString(fmt.Sprintf("  %d: %s\n", s+i, v))
		}
		return b.String(), nil

	case "set_add":
		key := strArg(in, "key")
		if key == "" {
			return "", fmt.Errorf("missing key")
		}
		member := strArg(in, "member", "value")
		if member == "" {
			return "", fmt.Errorf("missing member")
		}
		n, err := client.SAdd(ctx, key, member).Result()
		if err != nil {
			return "", fmt.Errorf("set_add failed: %w", err)
		}
		return fmt.Sprintf("SADD %s: added %d member(s)", key, n), nil

	case "set_members":
		key := strArg(in, "key")
		if key == "" {
			return "", fmt.Errorf("missing key")
		}
		members, err := client.SMembers(ctx, key).Result()
		if err != nil {
			return "", fmt.Errorf("set_members failed: %w", err)
		}
		if len(members) == 0 {
			return fmt.Sprintf("Set '%s' is empty", key), nil
		}
		return fmt.Sprintf("SMEMBERS %s (%d):\n%s", key, len(members), strings.Join(members, "\n")), nil

	case "zset_add":
		key := strArg(in, "key")
		if key == "" {
			return "", fmt.Errorf("missing key")
		}
		member := strArg(in, "member", "value")
		if member == "" {
			return "", fmt.Errorf("missing member")
		}
		score := strArg(in, "score")
		if score == "" {
			return "", fmt.Errorf("missing score")
		}
		var s float64
		if _, err := fmt.Sscanf(score, "%f", &s); err != nil {
			return "", fmt.Errorf("invalid score: %s", score)
		}
		n, err := client.ZAdd(ctx, key, redis.Z{Score: s, Member: member}).Result()
		if err != nil {
			return "", fmt.Errorf("zset_add failed: %w", err)
		}
		return fmt.Sprintf("ZADD %s: added %d member(s)", key, n), nil

	case "zset_range":
		key := strArg(in, "key")
		if key == "" {
			return "", fmt.Errorf("missing key")
		}
		start := strArg(in, "start")
		end := strArg(in, "end")
		s, e := int64(0), int64(-1)
		if start != "" {
			fmt.Sscanf(start, "%d", &s)
		}
		if end != "" {
			fmt.Sscanf(end, "%d", &e)
		}
		withscores := strArg(in, "with_scores", "scores")
		z, err := client.ZRange(ctx, key, s, e).Result()
		if err != nil {
			return fmt.Sprintf("ZRANGE %s:\n%s", key, z), nil
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("ZRANGE %s:\n", key))
		for i, m := range z {
			if withscores == "true" || withscores == "1" {
				sc, _ := client.ZScore(ctx, key, m).Result()
				b.WriteString(fmt.Sprintf("  %d: %s (score: %.2f)\n", s+int64(i), m, sc))
			} else {
				b.WriteString(fmt.Sprintf("  %d: %s\n", s+int64(i), m))
			}
		}
		return b.String(), nil

	case "info":
		section := strArg(in, "section", "target")
		var info string
		var err error
		if section != "" {
			info, err = client.Info(ctx, section).Result()
		} else {
			info, err = client.Info(ctx).Result()
		}
		if err != nil {
			return "", fmt.Errorf("info failed: %w", err)
		}
		return "INFO:\n" + info, nil

	case "dbsize":
		size, err := client.DBSize(ctx).Result()
		if err != nil {
			return "", fmt.Errorf("dbsize failed: %w", err)
		}
		return fmt.Sprintf("DBSIZE: %d keys", size), nil

	case "ping":
		pong, err := client.Ping(ctx).Result()
		if err != nil {
			return "", fmt.Errorf("ping failed: %w", err)
		}
		return "PONG: " + pong, nil

	default:
		return "", fmt.Errorf("unsupported operation: %s", op)
	}
}

func NewBuiltinRedisTool() tool.BaseTool {
	return toolutils.NewTool(
		&einoschema.ToolInfo{
			Name:  toolRedis,
			Desc:  "Redis operations: get, set, del, keys, hash, list, set, zset, info, dbsize, ping. Supports password auth and multiple databases.",
			Extra: map[string]any{"execution_mode": "client"},
			ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
				"operation":   {Type: einoschema.String, Desc: "Operation: get, set, del, keys, exists, type, ttl, expire, incr, decr, hash_get, hash_set, hash_del, hash_getall, list_push, list_range, set_add, set_members, zset_add, zset_range, info, dbsize, ping", Required: true},
				"addr":        {Type: einoschema.String, Desc: "Redis address (default: localhost:6379)", Required: false},
				"password":    {Type: einoschema.String, Desc: "Redis password (optional)", Required: false},
				"db":          {Type: einoschema.String, Desc: "Database number (default: 0)", Required: false},
				"key":         {Type: einoschema.String, Desc: "Key for string/hash/list/set/zset operations", Required: false},
				"value":       {Type: einoschema.String, Desc: "Value to set", Required: false},
				"field":       {Type: einoschema.String, Desc: "Field for hash operations", Required: false},
				"ttl":         {Type: einoschema.String, Desc: "TTL in seconds for set/expire", Required: false},
				"pattern":     {Type: einoschema.String, Desc: "Pattern for keys operation (default: *)", Required: false},
				"score":       {Type: einoschema.String, Desc: "Score for zset operations", Required: false},
				"member":      {Type: einoschema.String, Desc: "Member for set/zset operations", Required: false},
				"direction":   {Type: einoschema.String, Desc: "List push direction: left/right (default: left)", Required: false},
				"start":       {Type: einoschema.String, Desc: "Start index for list_range/zset_range", Required: false},
				"end":         {Type: einoschema.String, Desc: "End index for list_range/zset_range", Required: false},
				"with_scores": {Type: einoschema.String, Desc: "Include scores in zset_range (true/1 or false)", Required: false},
				"section":     {Type: einoschema.String, Desc: "Info section (e.g., memory, clients)", Required: false},
			}),
		},
		execBuiltinRedisTool,
	)
}
