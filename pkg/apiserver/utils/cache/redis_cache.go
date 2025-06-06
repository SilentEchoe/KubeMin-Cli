package cache

import (
	"context"
	"github.com/redis/go-redis/v9"
	"time"
)

type RedisCache struct {
	redisClient *redis.Client
}

var redisClient *redis.Client

// NewRedisCache callers has to make sure the caller has the settings for redis in their env variables.
func NewRedisCache(db int) *RedisCache {
	//if redisClient == nil {
	//	redisConfig := &redis.Options{
	//		Addr:     fmt.Sprintf("%s:%d", "", config.RedisPort),
	//		DB:       db,
	//		Password: config.RedisPassword,
	//		Username: config.RedisUserName,
	//	}
	//
	//	redisClient = redis.NewClient(redisConfig)
	//}
	return &RedisCache{redisClient: redisClient}
}

func (c *RedisCache) Write(key, val string, ttl time.Duration) error {
	_, err := c.redisClient.Set(context.TODO(), key, val, ttl).Result()
	return err
}

func (c *RedisCache) HWrite(key, field, val string, ttl time.Duration) error {
	_, err := c.redisClient.HSet(context.TODO(), key, field, val).Result()
	if err != nil {
		return err
	}

	// not thread safe
	if ttl > 0 {
		_, err = c.redisClient.Expire(context.Background(), key, ttl).Result()
	}
	return err
}

func (c *RedisCache) SetNX(key, val string, ttl time.Duration) error {
	_, err := c.redisClient.SetNX(context.TODO(), key, val, ttl).Result()
	return err
}

func (c *RedisCache) Exists(key string) (bool, error) {
	exists, err := c.redisClient.Exists(context.TODO(), key).Result()
	if err != nil {
		return false, err
	}

	if exists == 1 {
		return true, nil
	} else {
		return false, nil
	}
}

func (c *RedisCache) GetString(key string) (string, error) {
	return c.redisClient.Get(context.TODO(), key).Result()
}

func (c *RedisCache) HGetString(key, field string) (string, error) {
	return c.redisClient.HGet(context.TODO(), key, field).Result()
}

func (c *RedisCache) HGetAllString(key string) (map[string]string, error) {
	return c.redisClient.HGetAll(context.Background(), key).Result()
}

func (c *RedisCache) Delete(key string) error {
	return c.redisClient.Del(context.TODO(), key).Err()
}

func (c *RedisCache) HDelete(key, field string) error {
	_, err := c.redisClient.HDel(context.Background(), key, field).Result()
	return err
}

func (c *RedisCache) Publish(channel, message string) error {
	return c.redisClient.Publish(context.Background(), channel, message).Err()
}

func (c *RedisCache) Subscribe(channel string) (<-chan *redis.Message, func() error) {
	sub := c.redisClient.Subscribe(context.Background(), channel)
	return sub.Channel(), sub.Close
}

func (c *RedisCache) FlushDBAsync() error {
	return c.redisClient.FlushDBAsync(context.Background()).Err()
}

func (c *RedisCache) ListSetMembers(key string) ([]string, error) {
	return c.redisClient.SMembers(context.Background(), key).Result()
}

func (c *RedisCache) AddElementsToSet(key string, elements []string, ttl time.Duration) error {
	if len(elements) == 0 {
		return nil
	}
	err := c.redisClient.SAdd(context.Background(), key, elements).Err()
	if err != nil {
		return err
	}
	c.redisClient.Expire(context.Background(), key, ttl)
	return nil
}

func (c *RedisCache) RemoveElementsFromSet(key string, elements []string) error {
	if len(elements) == 0 {
		return nil
	}
	return c.redisClient.SRem(context.Background(), key, elements).Err()
}

type RedisCacheAI struct {
	redisClient *redis.Client
	ttl         time.Duration
	noCache     bool
	hashKey     string
}
