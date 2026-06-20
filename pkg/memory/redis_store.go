package memory

import (
    "context"
    "encoding/json"
    "fmt"
    "strings"

    "github.com/redis/go-redis/v9"
    "github.com/sipeed/picoclaw/pkg/providers"
)

// RedisStore implements Store using Redis lists and hashes.
// Intended for durable remote session storage (Upstash, etc.).
type RedisStore struct {
    rdb *redis.Client
}

// NewRedisStore creates a Redis-backed Store from a redis URL (rediss://...)
func NewRedisStore(redisURL string) (*RedisStore, error) {
    opt, err := redis.ParseURL(strings.TrimSpace(redisURL))
    if err != nil {
        return nil, fmt.Errorf("memory: parse redis url: %w", err)
    }
    rdb := redis.NewClient(opt)
    // ping to verify
    if err := rdb.Ping(context.Background()).Err(); err != nil {
        return nil, fmt.Errorf("memory: redis ping: %w", err)
    }
    return &RedisStore{rdb: rdb}, nil
}

func sanitizeRedisKey(key string) string {
    // mirror sanitizeKey behaviour for safety
    s := strings.ReplaceAll(key, ":", "_")
    s = strings.ReplaceAll(s, "/", "_")
    s = strings.ReplaceAll(s, "\\", "_")
    return s
}

func messagesKey(key string) string { return "sessions:msgs:" + sanitizeRedisKey(key) }
func metaKey(key string) string     { return "sessions:meta:" + sanitizeRedisKey(key) }
func indexKey() string             { return "sessions:index" }

func (s *RedisStore) AddMessage(ctx context.Context, sessionKey, role, content string) error {
    msg := providers.Message{Role: role, Content: content}
    return s.AddFullMessage(ctx, sessionKey, msg)
}

func (s *RedisStore) AddFullMessage(ctx context.Context, sessionKey string, msg providers.Message) error {
    b, err := json.Marshal(msg)
    if err != nil {
        return fmt.Errorf("memory: marshal message: %w", err)
    }
    if err := s.rdb.RPush(ctx, messagesKey(sessionKey), b).Err(); err != nil {
        return fmt.Errorf("memory: rpush: %w", err)
    }
    if err := s.rdb.SAdd(ctx, indexKey(), sessionKey).Err(); err != nil {
        // non-fatal but return error so callers can log
        return fmt.Errorf("memory: add index: %w", err)
    }
    return nil
}

func (s *RedisStore) GetHistory(ctx context.Context, sessionKey string) ([]providers.Message, error) {
    vals, err := s.rdb.LRange(ctx, messagesKey(sessionKey), 0, -1).Result()
    if err != nil && err != redis.Nil {
        return nil, fmt.Errorf("memory: lrange: %w", err)
    }
    out := make([]providers.Message, 0, len(vals))
    for _, v := range vals {
        var msg providers.Message
        if err := json.Unmarshal([]byte(v), &msg); err != nil {
            // skip corrupt entries
            continue
        }
        out = append(out, msg)
    }
    return out, nil
}

func (s *RedisStore) GetSummary(ctx context.Context, sessionKey string) (string, error) {
    v, err := s.rdb.HGet(ctx, metaKey(sessionKey), "summary").Result()
    if err == redis.Nil {
        return "", nil
    }
    if err != nil {
        return "", fmt.Errorf("memory: hget summary: %w", err)
    }
    return v, nil
}

func (s *RedisStore) SetSummary(ctx context.Context, sessionKey, summary string) error {
    if err := s.rdb.HSet(ctx, metaKey(sessionKey), "summary", summary).Err(); err != nil {
        return fmt.Errorf("memory: hset summary: %w", err)
    }
    if err := s.rdb.SAdd(ctx, indexKey(), sessionKey).Err(); err != nil {
        return fmt.Errorf("memory: add index: %w", err)
    }
    return nil
}

func (s *RedisStore) TruncateHistory(ctx context.Context, sessionKey string, keepLast int) error {
    if keepLast <= 0 {
        if err := s.rdb.Del(ctx, messagesKey(sessionKey)).Err(); err != nil {
            return fmt.Errorf("memory: del messages: %w", err)
        }
        return nil
    }
    // Keep last N -> LTRIM -N  -1
    start := -keepLast
    if err := s.rdb.LTrim(ctx, messagesKey(sessionKey), int64(start), -1).Err(); err != nil {
        return fmt.Errorf("memory: ltrim: %w", err)
    }
    return nil
}

func (s *RedisStore) SetHistory(ctx context.Context, sessionKey string, history []providers.Message) error {
    // replace list atomically via MULTI
    pipe := s.rdb.TxPipeline()
    pipe.Del(ctx, messagesKey(sessionKey))
    if len(history) > 0 {
        vals := make([]interface{}, 0, len(history))
        for _, m := range history {
            b, err := json.Marshal(m)
            if err != nil {
                return fmt.Errorf("memory: marshal history: %w", err)
            }
            vals = append(vals, b)
        }
        pipe.RPush(ctx, messagesKey(sessionKey), vals...)
    }
    pipe.SAdd(ctx, indexKey(), sessionKey)
    if _, err := pipe.Exec(ctx); err != nil {
        return fmt.Errorf("memory: set history exec: %w", err)
    }
    return nil
}

func (s *RedisStore) Compact(ctx context.Context, sessionKey string) error {
    // Redis lists do not accumulate dead data in the same way; no-op.
    return nil
}

func (s *RedisStore) ListSessions() []string {
    vals, err := s.rdb.SMembers(context.Background(), indexKey()).Result()
    if err != nil {
        return nil
    }
    return vals
}

func (s *RedisStore) Close() error {
    return s.rdb.Close()
}
