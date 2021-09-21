package caching

import (
	"log"
	"time"

	"github.com/adelowo/onecache"
	"github.com/adelowo/onecache/redis"
	r "github.com/go-redis/redis"
)

type Cacheable interface {
	// SerializeSelf directs a cacheable object to serialize itself for caching.
	SerializeSelf() ([]byte, error)

	// DeserializeSelf directs a cacheable object to restore itself from the
	// provided string created with SerializeSelf.
	DeserializeSelf([]byte) error
}

type Cache struct {
	store onecache.Store
}

// NewCache creates a new cache.
func NewCache(addr string) *Cache {
	opts := &r.Options{
		Addr: addr,
		OnConnect: func(*r.Conn) error {
			log.Println("Connected to Redis server")
			return nil
		},
	}

	store := redis.New(redis.ClientOptions(opts))

	return &Cache{
		store: store,
	}
}

// Has returns true if the provided key is in the cache.
func (c *Cache) Has(key string) bool {
	return c.store.Has(key)
}

// Set serializes the provided Cacheable and saves it to the cache for the provided
// amount of time.
func (c *Cache) Set(key string, o Cacheable, expires time.Duration) error {
	data, err := o.SerializeSelf()
	if err != nil {
		return err
	}

	err = c.store.Set(key, data, expires)
	if err != nil {
		return err
	}

	return nil
}

// Get retrieves a Cacheable's data from the cache and calls the cacheable's
// DeserializeSelf function.
func (c *Cache) Get(key string, o Cacheable) error {
	data, err := c.store.Get(key)
	if err != nil {
		return err
	}

	err = o.DeserializeSelf(data)
	if err != nil {
		return err
	}

	return nil
}
