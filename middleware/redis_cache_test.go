package middleware

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testFixture struct {
	mr     *miniredis.Miniredis
	client *redis.Client
	mw     *RedisCacheMiddleware
	router *gin.Engine
}

func newFixture(t *testing.T) *testFixture {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{
		Addr:     mr.Addr(),
		PoolSize: 1,
	})
	mw := NewRedisCacheMiddleware(client, WithTimeout(100*time.Millisecond))

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(mw.Middleware())

	t.Cleanup(func() {
		client.Close()
		mr.Close()
	})

	return &testFixture{mr, client, mw, router}
}

func cacheEntry(t *testing.T, status int, contentType string, body string) string {
	entry := cachedResponse{
		Status:      status,
		ContentType: contentType,
		Body:        []byte(body),
	}
	data, err := json.Marshal(entry)
	require.NoError(t, err)
	return string(data)
}

func TestCacheHit(t *testing.T) {
	fx := newFixture(t)

	fx.mr.Set("GET:/restaurants", cacheEntry(t, 200, "application/json", `{"data":"cached"}`))

	handlerCalled := false
	fx.router.GET("/restaurants", func(c *gin.Context) {
		handlerCalled = true
		c.JSON(200, gin.H{"data": "fresh"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/restaurants", nil)
	fx.router.ServeHTTP(w, req)

	assert.False(t, handlerCalled, "handler should not be called on cache hit")
	assert.Equal(t, 200, w.Code)
	assert.JSONEq(t, `{"data":"cached"}`, w.Body.String())
}

func TestCacheMiss(t *testing.T) {
	fx := newFixture(t)

	fx.router.GET("/restaurants", func(c *gin.Context) {
		c.JSON(200, gin.H{"data": "fresh"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/restaurants", nil)
	fx.router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	assert.JSONEq(t, `{"data":"fresh"}`, w.Body.String())

	stored, err := fx.mr.Get("GET:/restaurants")
	require.NoError(t, err, "response should be cached")
	var entry cachedResponse
	err = json.Unmarshal([]byte(stored), &entry)
	require.NoError(t, err)
	assert.Equal(t, 200, entry.Status)
	assert.Equal(t, `{"data":"fresh"}`, string(entry.Body))
}

func TestCacheHit_RespectsStatusAndContentType(t *testing.T) {
	fx := newFixture(t)

	fx.mr.Set("GET:/menus/5", cacheEntry(t, 201, "text/plain", "hello"))

	fx.router.GET("/menus/:id", func(c *gin.Context) {
		c.JSON(200, gin.H{"id": c.Param("id")})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/menus/5", nil)
	fx.router.ServeHTTP(w, req)

	assert.Equal(t, 201, w.Code)
	assert.Equal(t, "text/plain", w.Header().Get("Content-Type"))
	assert.Equal(t, "hello", w.Body.String())
}

func TestSkipPaths(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{Addr: mr.Addr(), PoolSize: 1})
	defer client.Close()

	mw := NewRedisCacheMiddleware(client,
		WithSkipPaths("/restaurants"),
		WithTimeout(100*time.Millisecond),
	)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(mw.Middleware())

	mr.Set("GET:/restaurants", cacheEntry(t, 200, "application/json", `{"data":"cached"}`))

	handlerCalled := false
	router.GET("/restaurants", func(c *gin.Context) {
		handlerCalled = true
		c.JSON(200, gin.H{"data": "fresh"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/restaurants", nil)
	router.ServeHTTP(w, req)

	assert.True(t, handlerCalled, "handler should execute for skipped path")
	assert.JSONEq(t, `{"data":"fresh"}`, w.Body.String())
}

func TestNonGET_NoCache(t *testing.T) {
	fx := newFixture(t)

	fx.mr.Set("GET:/restaurants", cacheEntry(t, 200, "application/json", `{"data":"cached"}`))

	handlerCalled := false
	fx.router.POST("/restaurants", func(c *gin.Context) {
		handlerCalled = true
		c.JSON(201, gin.H{"id": "new"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/restaurants", nil)
	fx.router.ServeHTTP(w, req)

	assert.True(t, handlerCalled, "handler should execute for POST")
	assert.Equal(t, 201, w.Code)
}

func TestRedisDown(t *testing.T) {
	gin.SetMode(gin.TestMode)

	client := redis.NewClient(&redis.Options{
		Addr:        "invalid:6379",
		DialTimeout: 10 * time.Millisecond,
		PoolSize:    1,
	})
	defer client.Close()

	mw := NewRedisCacheMiddleware(client, WithTimeout(50*time.Millisecond))

	router := gin.New()
	router.Use(mw.Middleware())

	handlerCalled := false
	router.GET("/restaurants", func(c *gin.Context) {
		handlerCalled = true
		c.JSON(200, gin.H{"data": "fresh"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/restaurants", nil)
	router.ServeHTTP(w, req)

	assert.True(t, handlerCalled, "handler should execute when Redis is down")
	assert.Equal(t, 200, w.Code)
	assert.JSONEq(t, `{"data":"fresh"}`, w.Body.String())
}

func TestCacheHit_BadCacheEntry(t *testing.T) {
	fx := newFixture(t)

	fx.mr.Set("GET:/restaurants", "not-valid-json")

	handlerCalled := false
	fx.router.GET("/restaurants", func(c *gin.Context) {
		handlerCalled = true
		c.JSON(200, gin.H{"data": "fresh"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/restaurants", nil)
	fx.router.ServeHTTP(w, req)

	assert.True(t, handlerCalled, "handler should execute when cache entry is corrupt")
	assert.JSONEq(t, `{"data":"fresh"}`, w.Body.String())
}

func TestInvalidation_POST_Restaurants(t *testing.T) {
	fx := newFixture(t)

	fx.mr.Set("GET:/restaurants", cacheEntry(t, 200, "application/json", `["old"]`))

	handlerCalled := false
	fx.router.POST("/restaurants", func(c *gin.Context) {
		handlerCalled = true
		c.JSON(201, gin.H{"id": "new"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/restaurants", nil)
	fx.router.ServeHTTP(w, req)

	assert.True(t, handlerCalled)
	assert.Equal(t, 201, w.Code)

	assert.False(t, fx.mr.Exists("GET:/restaurants"),
		"restaurants cache should be invalidated after POST")
}

func TestInvalidation_POST_Menus(t *testing.T) {
	fx := newFixture(t)

	fx.mr.Set("GET:/menus", cacheEntry(t, 200, "application/json", `[{"id":1}]`))
	fx.mr.Set("GET:/menus/1", cacheEntry(t, 200, "application/json", `{"id":1}`))
	fx.mr.Set("GET:/restaurants", cacheEntry(t, 200, "application/json", `["r1"]`))

	handlerCalled := false
	fx.router.POST("/menus", func(c *gin.Context) {
		handlerCalled = true
		c.JSON(201, gin.H{"id": "new"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/menus", nil)
	fx.router.ServeHTTP(w, req)

	assert.True(t, handlerCalled)

	assert.False(t, fx.mr.Exists("GET:/menus"), "menus list should be invalidated")
	assert.False(t, fx.mr.Exists("GET:/menus/1"), "menus detail should be invalidated")
	assert.True(t, fx.mr.Exists("GET:/restaurants"), "restaurants cache should remain intact")
}

func TestInvalidation_PUT_MenuByID(t *testing.T) {
	fx := newFixture(t)

	fx.mr.Set("GET:/menus", cacheEntry(t, 200, "application/json", `[{"id":5}]`))
	fx.mr.Set("GET:/menus/5", cacheEntry(t, 200, "application/json", `{"id":5,"name":"old"}`))
	fx.mr.Set("GET:/restaurants", cacheEntry(t, 200, "application/json", `["r1"]`))

	handlerCalled := false
	fx.router.PUT("/menus/:id", func(c *gin.Context) {
		handlerCalled = true
		assert.Equal(t, "5", c.Param("id"))
		c.JSON(200, gin.H{"id": 5, "name": "updated"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/menus/5", nil)
	fx.router.ServeHTTP(w, req)

	assert.True(t, handlerCalled)

	assert.False(t, fx.mr.Exists("GET:/menus"), "menus list should be invalidated")
	assert.False(t, fx.mr.Exists("GET:/menus/5"), "specific menu should be invalidated")
	assert.True(t, fx.mr.Exists("GET:/restaurants"), "restaurants cache should remain intact")
}

func TestInvalidation_DELETE_MenuByID(t *testing.T) {
	fx := newFixture(t)

	fx.mr.Set("GET:/menus", cacheEntry(t, 200, "application/json", `[{"id":3}]`))
	fx.mr.Set("GET:/menus/3", cacheEntry(t, 200, "application/json", `{"id":3}`))

	fx.router.DELETE("/menus/:id", func(c *gin.Context) {
		assert.Equal(t, "3", c.Param("id"))
		c.JSON(200, gin.H{"deleted": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("DELETE", "/menus/3", nil)
	fx.router.ServeHTTP(w, req)

	assert.False(t, fx.mr.Exists("GET:/menus"), "menus list should be invalidated")
	assert.False(t, fx.mr.Exists("GET:/menus/3"), "specific menu should be invalidated")
}

func TestInvalidation_SkipsOnNon2xx(t *testing.T) {
	fx := newFixture(t)

	fx.mr.Set("GET:/restaurants", cacheEntry(t, 200, "application/json", `["data"]`))

	fx.router.POST("/restaurants", func(c *gin.Context) {
		c.JSON(400, gin.H{"error": "bad request"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/restaurants", nil)
	fx.router.ServeHTTP(w, req)

	assert.True(t, fx.mr.Exists("GET:/restaurants"),
		"cache should NOT be invalidated on non-2xx response")
}

func TestNilClient_DoesNothing(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mw := NewRedisCacheMiddleware(nil)

	router := gin.New()
	router.Use(mw.Middleware())

	handlerCalled := false
	router.GET("/restaurants", func(c *gin.Context) {
		handlerCalled = true
		c.JSON(200, gin.H{"data": "fresh"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/restaurants", nil)
	router.ServeHTTP(w, req)

	assert.True(t, handlerCalled)
	assert.JSONEq(t, `{"data":"fresh"}`, w.Body.String())
}

func TestInvalidation_UserRoutes(t *testing.T) {
	fx := newFixture(t)

	fx.mr.Set("GET:/users/me", cacheEntry(t, 200, "application/json", `{"id":"u1"}`))
	fx.mr.Set("GET:/users/u1", cacheEntry(t, 200, "application/json", `{"id":"u1","name":"old"}`))

	fx.router.PUT("/users/:id", func(c *gin.Context) {
		c.JSON(200, gin.H{"id": c.Param("id"), "name": "updated"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/users/u1", nil)
	fx.router.ServeHTTP(w, req)

	assert.False(t, fx.mr.Exists("GET:/users/u1"), "user cache should be invalidated")
	assert.False(t, fx.mr.Exists("GET:/users/me"), "users/me should be invalidated")
}

func TestInvalidation_NoRuleForRoute(t *testing.T) {
	fx := newFixture(t)

	fx.mr.Set("GET:/restaurants", cacheEntry(t, 200, "application/json", `["data"]`))

	fx.router.PATCH("/restaurants", func(c *gin.Context) {
		c.JSON(200, gin.H{"patched": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("PATCH", "/restaurants", nil)
	fx.router.ServeHTTP(w, req)

	assert.True(t, fx.mr.Exists("GET:/restaurants"),
		"cache should remain intact when no invalidation rule matches")
}

func TestInjectContextParams_DoesNotPanic(t *testing.T) {
	fx := newFixture(t)

	handlerCalled := false
	fx.router.GET("/users/:id/orders/:oid", func(c *gin.Context) {
		handlerCalled = true
		c.JSON(200, gin.H{"user": c.Param("id"), "order": c.Param("oid")})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/users/u1/orders/o2", nil)
	fx.router.ServeHTTP(w, req)

	assert.True(t, handlerCalled)
	assert.JSONEq(t, `{"user":"u1","order":"o2"}`, w.Body.String())
}
