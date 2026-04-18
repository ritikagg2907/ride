package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Client struct {
	rdb *redis.Client
}

func New(addr, password string, db int) *Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	return &Client{rdb: rdb}
}

func (c *Client) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

func (c *Client) Set(ctx context.Context, key string, val any, ttl time.Duration) error {
	b, err := json.Marshal(val)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, key, b, ttl).Err()
}

func (c *Client) Get(ctx context.Context, key string, dest any) error {
	b, err := c.rdb.Get(ctx, key).Bytes()
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dest)
}

func (c *Client) SetRaw(ctx context.Context, key, val string, ttl time.Duration) error {
	return c.rdb.Set(ctx, key, val, ttl).Err()
}

func (c *Client) GetRaw(ctx context.Context, key string) (string, error) {
	return c.rdb.Get(ctx, key).Result()
}

// SetNX sets only if key does not exist. Returns true if set.
func (c *Client) SetNX(ctx context.Context, key, val string, ttl time.Duration) (bool, error) {
	return c.rdb.SetNX(ctx, key, val, ttl).Result()
}

func (c *Client) Del(ctx context.Context, keys ...string) error {
	return c.rdb.Del(ctx, keys...).Err()
}

func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	n, err := c.rdb.Exists(ctx, key).Result()
	return n > 0, err
}

func (c *Client) Incr(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	pipe := c.rdb.Pipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return incr.Val(), nil
}

// GeoAdd adds a member to the geo sorted set.
func (c *Client) GeoAdd(ctx context.Context, key string, lng, lat float64, member string) error {
	return c.rdb.GeoAdd(ctx, key, &redis.GeoLocation{
		Name:      member,
		Longitude: lng,
		Latitude:  lat,
	}).Err()
}

// GeoRadius returns members within radiusKm of (lng,lat).
func (c *Client) GeoRadius(ctx context.Context, key string, lng, lat, radiusKm float64, count int) ([]redis.GeoLocation, error) {
	return c.rdb.GeoSearchLocation(ctx, key, &redis.GeoSearchLocationQuery{
		GeoSearchQuery: redis.GeoSearchQuery{
			Longitude:  lng,
			Latitude:   lat,
			Radius:     radiusKm,
			RadiusUnit: "km",
			Sort:       "ASC",
			Count:      count,
		},
		WithCoord: true,
		WithDist:  true,
	}).Result()
}

func (c *Client) GeoRemove(ctx context.Context, key, member string) error {
	return c.rdb.ZRem(ctx, key, member).Err()
}

// HSet sets fields on a hash.
func (c *Client) HSet(ctx context.Context, key string, fields map[string]any) error {
	args := make([]any, 0, len(fields)*2)
	for k, v := range fields {
		args = append(args, k, fmt.Sprintf("%v", v))
	}
	return c.rdb.HSet(ctx, key, args...).Err()
}

func (c *Client) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return c.rdb.HGetAll(ctx, key).Result()
}

// Redlock: simple single-node distributed lock using SetNX.
func (c *Client) LockAcquire(ctx context.Context, key, token string, ttl time.Duration) (bool, error) {
	return c.rdb.SetNX(ctx, "lock:"+key, token, ttl).Result()
}

func (c *Client) LockRelease(ctx context.Context, key, token string) error {
	script := `if redis.call("get", KEYS[1]) == ARGV[1] then return redis.call("del", KEYS[1]) else return 0 end`
	return c.rdb.Eval(ctx, script, []string{"lock:" + key}, token).Err()
}

// Publish sends a message to a Redis pub/sub channel.
func (c *Client) Publish(ctx context.Context, channel, msg string) error {
	return c.rdb.Publish(ctx, channel, msg).Err()
}

func (c *Client) Subscribe(ctx context.Context, channel string) *redis.PubSub {
	return c.rdb.Subscribe(ctx, channel)
}

func (c *Client) Scan(ctx context.Context, pattern string) ([]string, error) {
	var keys []string
	iter := c.rdb.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	return keys, iter.Err()
}
