package proxy

import (
	"context"
	"fmt"

	"github.com/go-redis/redis/v8"
)

type RedisStorage struct {
	redis  *redis.Client
	prefix string
}

func NewRedisStorage(redis *redis.Client, prefix string) *RedisStorage {
	return &RedisStorage{
		redis:  redis,
		prefix: prefix,
	}
}

func (s *RedisStorage) key(d *URLData) string {
	return fmt.Sprintf("storage:%s:%s", s.prefix, d.Sha256)
}

func (s *RedisStorage) Load(data *URLData) ([]byte, error) {
	res, err := s.redis.Get(context.TODO(), s.key(data)).Result()
	if err != nil {
		return nil, err
	}
	return []byte(res), nil
}

func (s *RedisStorage) Store(data *URLData, content []byte) error {
	return s.redis.Set(context.TODO(), s.key(data), content, 0).Err()
}
