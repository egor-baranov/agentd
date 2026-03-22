package session

import (
	"context"
	"strings"

	"github.com/redis/go-redis/v9"
)

type Opened struct {
	Broker Broker
	Close  func() error
}

func Open(ctx context.Context, redisAddr string) (*Opened, error) {
	if redisAddr == "" {
		return &Opened{Broker: NewMemoryBroker(), Close: func() error { return nil }}, nil
	}
	opts, err := redis.ParseURL(redisAddr)
	if err != nil {
		opts = &redis.Options{Addr: strings.TrimPrefix(redisAddr, "redis://")}
	}
	client := redis.NewClient(opts)
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}
	return &Opened{Broker: NewRedisBroker(client, ""), Close: client.Close}, nil
}
