package handlers_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"restaurante/handlers"
	"restaurante/mocks"
	"restaurante/models"

	"github.com/Nerzal/gocloak/v13"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// helper: crea un router de prueba con gin en modo test
func setupRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return gin.New()
}

// helper: serializa body a JSON y crea un request HTTP
func newJSONRequest(method, url string, body interface{}) *http.Request {
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(method, url, bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json")
	return req
}

// helper: inyecta user_id en el contexto de gin (simula middleware de auth)
func withUserID(c *gin.Context, userID string) {
	c.Set("user_id", userID)
}

// ── GetCurrentUser ────────────────────────────────────────────────────────────

func TestGetCurrentUser_OK(t *testing.T) {
	mockUsers := new(mocks.MockUserRepository)
	mockUsers.On("GetByID", "user-123").Return(&models.User{
		ID:     "user-123",
		Nombre: "Ana López",
		Email:  "ana@mail.com",
	}, nil)

	r := setupRouter()
	r.GET("/users/me", func(c *gin.Context) {
		c.Set("user_id", "user-123")
		handlers.GetCurrentUser(mockUsers)(c)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newJSONRequest("GET", "/users/me", nil))

	assert.Equal(t, http.StatusOK, w.Code)
	var resp models.User
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "user-123", resp.ID)
	assert.Equal(t, "Ana López", resp.Nombre)
	mockUsers.AssertExpectations(t)
}

func TestGetCurrentUser_NotFound(t *testing.T) {
	mockUsers := new(mocks.MockUserRepository)
	mockUsers.On("GetByID", "user-999").Return(nil, errors.New("not found"))

	r := setupRouter()
	r.GET("/users/me", func(c *gin.Context) {
		c.Set("user_id", "user-999")
		handlers.GetCurrentUser(mockUsers)(c)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newJSONRequest("GET", "/users/me", nil))

	assert.Equal(t, http.StatusNotFound, w.Code)
	mockUsers.AssertExpectations(t)
}

// ── UpdateUser ────────────────────────────────────────────────────────────────

func TestUpdateUser_OK(t *testing.T) {
	mockUsers := new(mocks.MockUserRepository)
	mockUsers.On("Update", "user-123", "Nuevo Nombre", "nuevo@mail.com").Return(nil)

	r := setupRouter()
	r.PUT("/users/:id", func(c *gin.Context) {
		c.Set("user_id", "user-123")
		handlers.UpdateUser(mockUsers)(c)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newJSONRequest("PUT", "/users/user-123", map[string]string{
		"nombre": "Nuevo Nombre",
		"email":  "nuevo@mail.com",
	}))

	assert.Equal(t, http.StatusOK, w.Code)
	mockUsers.AssertExpectations(t)
}

func TestUpdateUser_Forbidden(t *testing.T) {
	// user-123 intenta modificar a user-456
	mockUsers := new(mocks.MockUserRepository)

	r := setupRouter()
	r.PUT("/users/:id", func(c *gin.Context) {
		c.Set("user_id", "user-123")
		handlers.UpdateUser(mockUsers)(c)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newJSONRequest("PUT", "/users/user-456", map[string]string{
		"nombre": "Hack",
	}))

	assert.Equal(t, http.StatusForbidden, w.Code)
	// no se debe haber llamado Update
	mockUsers.AssertNotCalled(t, "Update")
}

func TestUpdateUser_DBError(t *testing.T) {
	mockUsers := new(mocks.MockUserRepository)
	mockUsers.On("Update", "user-123", "Nombre", "").Return(errors.New("db error"))

	r := setupRouter()
	r.PUT("/users/:id", func(c *gin.Context) {
		c.Set("user_id", "user-123")
		handlers.UpdateUser(mockUsers)(c)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newJSONRequest("PUT", "/users/user-123", map[string]string{
		"nombre": "Nombre",
	}))

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	mockUsers.AssertExpectations(t)
}

// ── DeleteUser ────────────────────────────────────────────────────────────────

func TestDeleteUser_OK(t *testing.T) {
	mockUsers := new(mocks.MockUserRepository)
	mockUsers.On("Delete", "user-123").Return(nil)

	r := setupRouter()
	r.DELETE("/users/:id", func(c *gin.Context) {
		c.Set("user_id", "user-123")
		handlers.DeleteUser(mockUsers)(c)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newJSONRequest("DELETE", "/users/user-123", nil))

	assert.Equal(t, http.StatusOK, w.Code)
	mockUsers.AssertExpectations(t)
}

func TestDeleteUser_Forbidden(t *testing.T) {
	mockUsers := new(mocks.MockUserRepository)

	r := setupRouter()
	r.DELETE("/users/:id", func(c *gin.Context) {
		c.Set("user_id", "user-123")
		handlers.DeleteUser(mockUsers)(c)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newJSONRequest("DELETE", "/users/user-456", nil))

	assert.Equal(t, http.StatusForbidden, w.Code)
	mockUsers.AssertNotCalled(t, "Delete")
}

func TestDeleteUser_DBError(t *testing.T) {
	mockUsers := new(mocks.MockUserRepository)
	mockUsers.On("Delete", "user-123").Return(errors.New("db error"))

	r := setupRouter()
	r.DELETE("/users/:id", func(c *gin.Context) {
		c.Set("user_id", "user-123")
		handlers.DeleteUser(mockUsers)(c)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newJSONRequest("DELETE", "/users/user-123", nil))

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	mockUsers.AssertExpectations(t)
}

// ── RegisterUser ──────────────────────────────────────────────────────────────
// Login y RegisterUser dependen de keycloak.Client que no es una interfaz,
// por lo que se prueba únicamente la validación del body (sin instanciar Keycloak)

func TestRegisterUser_BadBody(t *testing.T) {
	// body vacío → ShouldBindJSON falla porque nombre, email y password son required
	mockUsers := new(mocks.MockUserRepository)

	r := setupRouter()
	// pasamos nil como keycloak.Client — nunca llega a usarlo porque falla el bind
	r.POST("/auth/register", handlers.RegisterUser(nil, mockUsers))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/auth/register", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRegisterUser_MissingFields(t *testing.T) {
	mockUsers := new(mocks.MockUserRepository)

	r := setupRouter()
	r.POST("/auth/register", handlers.RegisterUser(nil, mockUsers))

	// falta password
	w := httptest.NewRecorder()
	r.ServeHTTP(w, newJSONRequest("POST", "/auth/register", map[string]string{
		"nombre":   "Ana",
		"apellido": "López",
		"email":    "ana@mail.com",
	}))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ── Login ─────────────────────────────────────────────────────────────────────

func TestLogin_BadBody(t *testing.T) {
	r := setupRouter()
	// nil como keycloak.Client — falla el bind antes de usarlo
	r.POST("/auth/login", handlers.Login(nil))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/auth/login", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLogin_MissingPassword(t *testing.T) {
	r := setupRouter()
	r.POST("/auth/login", handlers.Login(nil))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newJSONRequest("POST", "/auth/login", map[string]string{
		"email": "ana@mail.com",
	}))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLogin_Success(t *testing.T) {
	mockKC := new(mocks.MockKeycloakClient)
	mockKC.On("LoginUser", "ana@mail.com", "pass123").Return(&gocloak.JWT{
		AccessToken:  "token-abc",
		RefreshToken: "refresh-xyz",
		ExpiresIn:    3600,
		TokenType:    "Bearer",
	}, nil)

	r := setupRouter()
	r.POST("/auth/login", handlers.Login(mockKC))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newJSONRequest("POST", "/auth/login", map[string]string{
		"email":    "ana@mail.com",
		"password": "pass123",
	}))

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "token-abc", resp["access_token"])
	assert.Equal(t, "refresh-xyz", resp["refresh_token"])
	assert.Equal(t, float64(3600), resp["expires_in"])
	mockKC.AssertExpectations(t)
}

func TestLogin_Unauthorized(t *testing.T) {
	mockKC := new(mocks.MockKeycloakClient)
	mockKC.On("LoginUser", "ana@mail.com", "wrong").Return(nil, errors.New("invalid credentials"))

	r := setupRouter()
	r.POST("/auth/login", handlers.Login(mockKC))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newJSONRequest("POST", "/auth/login", map[string]string{
		"email":    "ana@mail.com",
		"password": "wrong",
	}))

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	mockKC.AssertExpectations(t)
}

// ── RegisterUser ──────────────────────────────────────────────────────────

func TestRegisterUser_Success(t *testing.T) {
	mockKC := new(mocks.MockKeycloakClient)
	mockUsers := new(mocks.MockUserRepository)

	mockKC.On("RegisterUser", "Ana", "López", "ana@mail.com", "pass123").Return("kc-user-1", nil)
	mockUsers.On("UpsertFromToken", "kc-user-1", "Ana", "ana@mail.com", false).Return(nil)

	r := setupRouter()
	r.POST("/auth/register", handlers.RegisterUser(mockKC, mockUsers))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newJSONRequest("POST", "/auth/register", map[string]string{
		"nombre":   "Ana",
		"apellido": "López",
		"email":    "ana@mail.com",
		"password": "pass123",
	}))

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), "kc-user-1")
	mockKC.AssertExpectations(t)
	mockUsers.AssertExpectations(t)
}

func TestRegisterUser_KeycloakConflict(t *testing.T) {
	mockKC := new(mocks.MockKeycloakClient)
	mockKC.On("RegisterUser", "Ana", "López", "dup@mail.com", "pass123").Return("", errors.New("email ya registrado"))

	r := setupRouter()
	r.POST("/auth/register", handlers.RegisterUser(mockKC, nil))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newJSONRequest("POST", "/auth/register", map[string]string{
		"nombre":   "Ana",
		"apellido": "López",
		"email":    "dup@mail.com",
		"password": "pass123",
	}))

	assert.Equal(t, http.StatusConflict, w.Code)
	mockKC.AssertExpectations(t)
}

func TestRegisterUser_UpsertFails(t *testing.T) {
	mockKC := new(mocks.MockKeycloakClient)
	mockUsers := new(mocks.MockUserRepository)

	mockKC.On("RegisterUser", "Ana", "López", "ana@mail.com", "pass123").Return("kc-user-1", nil)
	mockUsers.On("UpsertFromToken", "kc-user-1", "Ana", "ana@mail.com", false).Return(errors.New("db error"))

	r := setupRouter()
	r.POST("/auth/register", handlers.RegisterUser(mockKC, mockUsers))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newJSONRequest("POST", "/auth/register", map[string]string{
		"nombre":   "Ana",
		"apellido": "López",
		"email":    "ana@mail.com",
		"password": "pass123",
	}))

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), "sincronizacion")
	mockKC.AssertExpectations(t)
	mockUsers.AssertExpectations(t)
}
