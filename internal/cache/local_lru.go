package cache

import (
	lru "github.com/hashicorp/golang-lru/v2"
)

type LocalCache struct {
	cache *lru.Cache[string, string]
}

func NewLocalCache(size int) (*LocalCache, error) {
	c, err := lru.New[string, string](size)
	if err != nil {
		return nil, err
	}
	return &LocalCache{cache: c}, nil
}

func (l *LocalCache) Get(key string) (string, bool) {
	return l.cache.Get(key)
}

func (l *LocalCache) Add(key, value string) {
	l.cache.Add(key, value)
}
