package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

const (
	defaultCacheDuration = 15 * time.Minute
	defaultRedisTimeout  = 500 * time.Millisecond
)

type cachedResponse struct {
	Status      int    `json:"s"`
	ContentType string `json:"c"`
	Body        []byte `json:"b,omitempty"`
}

type invalidationRule struct {
	exact []string // exact keys with optional :param placeholders
	scans []string // SCAN prefixes (e.g.: "GET:/menus")
}

var defaultInvalidationRules = map[string]invalidationRule{
	"POST /restaurants": {
		scans: []string{"GET:/restaurants"},
	},
	"POST /menus": {
		scans: []string{"GET:/menus"},
	},
	"PUT /menus/:id": {
		exact: []string{"GET:/menus/:id"},
		scans: []string{"GET:/menus"},
	},
	"DELETE /menus/:id": {
		exact: []string{"GET:/menus/:id"},
		scans: []string{"GET:/menus"},
	},
	"PUT /users/:id": {
		exact: []string{"GET:/users/:id", "GET:/users/me"},
	},
	"DELETE /users/:id": {
		exact: []string{"GET:/users/:id"},
		scans: []string{"GET:/users"},
	},
	"POST /reservations": {
		scans: []string{"GET:/restaurants"},
	},
	"DELETE /reservations/:id": {
		scans: []string{"GET:/restaurants"},
	},
	"POST /orders": {
		scans: []string{"GET:/restaurants"},
	},
}

// RedisCacheMiddleware is a Gin middleware that caches GET responses in Redis
// and invalidates the cache on mutation operations (POST/PUT/DELETE).
// Implements graceful degradation: if Redis fails, the API continues working.
type RedisCacheMiddleware struct {
	client            *redis.Client
	duration          time.Duration
	timeout           time.Duration
	skipPaths         []string
	invalidationRules map[string]invalidationRule
}

// NewRedisCacheMiddleware creates a new Redis cache middleware.
// Accepts functional options: WithDuration, WithTimeout, WithSkipPaths, WithInvalidationRules.
// If client is nil, the middleware operates in pass-through mode (does not cache).
func NewRedisCacheMiddleware(client *redis.Client, opts ...Option) *RedisCacheMiddleware {
	rules := make(map[string]invalidationRule, len(defaultInvalidationRules))
	for k, v := range defaultInvalidationRules {
		rules[k] = v
	}
	m := &RedisCacheMiddleware{
		client:            client,
		duration:          defaultCacheDuration,
		timeout:           defaultRedisTimeout,
		invalidationRules: rules,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// Option is a configuration function for RedisCacheMiddleware.
type Option func(*RedisCacheMiddleware)

// WithDuration sets the TTL for cache entries (default: 15 minutes).
func WithDuration(d time.Duration) Option {
	return func(m *RedisCacheMiddleware) {
		m.duration = d
	}
}

// WithTimeout sets the timeout for Redis operations (default: 500ms).
func WithTimeout(d time.Duration) Option {
	return func(m *RedisCacheMiddleware) {
		m.timeout = d
	}
}

// WithSkipPaths adds paths that are NOT cached (e.g.: "/auth/", "/users/").
func WithSkipPaths(paths ...string) Option {
	return func(m *RedisCacheMiddleware) {
		m.skipPaths = append(m.skipPaths, paths...)
	}
}

// WithInvalidationRules overrides the default invalidation rules.
func WithInvalidationRules(rules map[string]invalidationRule) Option {
	return func(m *RedisCacheMiddleware) {
		m.invalidationRules = rules
	}
}

type responseWriter struct {
	gin.ResponseWriter
	body   bytes.Buffer
	status int
}

func (w *responseWriter) Write(data []byte) (int, error) {
	w.body.Write(data)
	return w.ResponseWriter.Write(data)
}

func (w *responseWriter) WriteString(s string) (int, error) {
	w.body.WriteString(s)
	return w.ResponseWriter.WriteString(s)
}

func (w *responseWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (m *RedisCacheMiddleware) cacheKey(c *gin.Context) string {
	return c.Request.Method + ":" + c.Request.URL.Path
}

// Middleware returns the Gin middleware function.
// On GET: checks cache (cache hit → serves response, cache miss → executes handler and stores).
// On POST/PUT/DELETE: executes handler and invalidates related routes if response is 2xx.
func (m *RedisCacheMiddleware) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodGet {
			m.handleGET(c)
			return
		}

		c.Next()
		if m.client != nil && c.Writer.Status() >= 200 && c.Writer.Status() < 300 {
			m.invalidate(c)
		}
	}
}

func (m *RedisCacheMiddleware) handleGET(c *gin.Context) {
	path := c.Request.URL.Path
	for _, p := range m.skipPaths {
		if strings.HasPrefix(path, p) {
			c.Next()
			return
		}
	}

	if m.client == nil {
		c.Next()
		return
	}

	key := m.cacheKey(c)

	ctx, cancel := context.WithTimeout(c.Request.Context(), m.timeout)
	defer cancel()

	cached, err := m.client.Get(ctx, key).Bytes()
	if err == nil {
		var entry cachedResponse
		if json.Unmarshal(cached, &entry) == nil {
			c.Data(entry.Status, entry.ContentType, entry.Body)
			c.Abort()
			return
		}
	}
	if err != redis.Nil {
		log.Printf("redis cache get error: %v", err)
	}

	w := &responseWriter{ResponseWriter: c.Writer, status: http.StatusOK}
	c.Writer = w

	c.Next()

	if w.status >= 200 && w.status < 300 && w.body.Len() > 0 {
		entry := cachedResponse{
			Status:      w.status,
			ContentType: w.Header().Get("Content-Type"),
			Body:        w.body.Bytes(),
		}
		data, err := json.Marshal(entry)
		if err != nil {
			log.Printf("redis cache marshal error: %v", err)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
		defer cancel()
		if err := m.client.Set(ctx, key, data, m.duration).Err(); err != nil {
			log.Printf("redis cache set error: %v", err)
		}
	}
}

func (m *RedisCacheMiddleware) invalidate(c *gin.Context) {
	fullPath := c.FullPath()
	if fullPath == "" {
		return
	}
	ruleKey := c.Request.Method + " " + fullPath
	rule, ok := m.invalidationRules[ruleKey]
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
	defer cancel()

	for _, key := range rule.exact {
		resolved := resolveKey(key, c)
		if err := m.client.Del(ctx, resolved).Err(); err != nil {
			log.Printf("redis cache invalidation error (del %s): %v", resolved, err)
		}
	}

	for _, prefix := range rule.scans {
		if err := m.deleteByPrefix(ctx, prefix); err != nil {
			log.Printf("redis cache invalidation error (scan %s): %v", prefix, err)
		}
	}
}

func (m *RedisCacheMiddleware) deleteByPrefix(ctx context.Context, prefix string) error {
	var cursor uint64
	var keys []string
	var err error

	for {
		keys, cursor, err = m.client.Scan(ctx, cursor, prefix+"*", 100).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			m.client.Del(ctx, keys...)
		}
		if cursor == 0 {
			break
		}
	}
	return nil
}

func resolveKey(key string, c *gin.Context) string {
	for _, param := range c.Params {
		key = strings.ReplaceAll(key, ":"+param.Key, param.Value)
	}
	return key
}
