package handlers_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"restaurante/elasticsearch"
	"restaurante/handlers"
	"restaurante/mocks"
	"restaurante/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ── ListRestaurants ───────────────────────────────────────────────────────────

func TestListRestaurants_OK(t *testing.T) {
	mockRest := new(mocks.MockRestaurantRepository)
	mockRest.On("List").Return([]models.Restaurant{
		{ID: "r-1", Nombre: "La Trattoria", Direccion: "San José"},
		{ID: "r-2", Nombre: "El Fogón", Direccion: "Cartago"},
	}, nil)

	r := setupRouter()
	r.GET("/restaurants", handlers.ListRestaurants(mockRest))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newJSONRequest("GET", "/restaurants", nil))

	assert.Equal(t, http.StatusOK, w.Code)
	mockRest.AssertExpectations(t)
}

func TestListRestaurants_Empty(t *testing.T) {
	mockRest := new(mocks.MockRestaurantRepository)
	mockRest.On("List").Return([]models.Restaurant{}, nil)

	r := setupRouter()
	r.GET("/restaurants", handlers.ListRestaurants(mockRest))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newJSONRequest("GET", "/restaurants", nil))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "[]")
	mockRest.AssertExpectations(t)
}

func TestListRestaurants_DBError(t *testing.T) {
	mockRest := new(mocks.MockRestaurantRepository)
	mockRest.On("List").Return(nil, errors.New("db error"))

	r := setupRouter()
	r.GET("/restaurants", handlers.ListRestaurants(mockRest))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newJSONRequest("GET", "/restaurants", nil))

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	mockRest.AssertExpectations(t)
}

// ── CreateRestaurant ──────────────────────────────────────────────────────────

func TestCreateRestaurant_OK(t *testing.T) {

	mockRest := new(mocks.MockRestaurantRepository)
	mockRest.On("Create", "La Trattoria", "San José Av 2", "88881234", "user-123", 40).
		Return("rest-1", nil)

	r := setupRouter()
	r.POST("/restaurants", func(c *gin.Context) {
		c.Set("user_id", "user-123")
		handlers.CreateRestaurant(mockRest)(c)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newJSONRequest("POST", "/restaurants", map[string]interface{}{
		"nombre":          "La Trattoria",
		"direccion":       "San José Av 2",
		"telefono":        "88881234",
		"capacidad_total": 40,
	}))

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), "rest-1")
	mockRest.AssertExpectations(t)
}

func TestCreateRestaurant_BadBody(t *testing.T) {
	mockRest := new(mocks.MockRestaurantRepository)

	r := setupRouter()
	r.POST("/restaurants", func(c *gin.Context) {
		c.Set("user_id", "user-123")
		handlers.CreateRestaurant(mockRest)(c)
	})

	// falta direccion y capacidad_total
	w := httptest.NewRecorder()
	r.ServeHTTP(w, newJSONRequest("POST", "/restaurants", map[string]string{
		"nombre": "Sin Dirección",
	}))

	assert.Equal(t, http.StatusBadRequest, w.Code)
	mockRest.AssertNotCalled(t, "Create")
}

func TestCreateRestaurant_DBError(t *testing.T) {
	mockRest := new(mocks.MockRestaurantRepository)
	mockRest.On("Create", "La Trattoria", "San José", "", "user-123", 30).
		Return("", errors.New("db error"))

	r := setupRouter()
	r.POST("/restaurants", func(c *gin.Context) {
		c.Set("user_id", "user-123")
		handlers.CreateRestaurant(mockRest)(c)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newJSONRequest("POST", "/restaurants", map[string]interface{}{
		"nombre":          "La Trattoria",
		"direccion":       "San José",
		"capacidad_total": 30,
	}))

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	mockRest.AssertExpectations(t)
}

// ── GetMenu ───────────────────────────────────────────────────────────────────

func TestGetMenu_OK(t *testing.T) {
	mockMenu := new(mocks.MockMenuRepository)
	mockMenu.On("GetByRestaurant", "rest-1").Return([]models.MenuItem{
		{ID: "p-1", Nombre: "Pizza", Precio: 5000, Categoria: "comida"},
	}, nil)

	r := setupRouter()
	r.GET("/menus/:id", handlers.GetMenu(mockMenu))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newJSONRequest("GET", "/menus/rest-1", nil))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Pizza")
	mockMenu.AssertExpectations(t)
}

func TestGetMenu_Empty(t *testing.T) {
	mockMenu := new(mocks.MockMenuRepository)
	mockMenu.On("GetByRestaurant", "rest-1").Return([]models.MenuItem{}, nil)

	r := setupRouter()
	r.GET("/menus/:id", handlers.GetMenu(mockMenu))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newJSONRequest("GET", "/menus/rest-1", nil))

	assert.Equal(t, http.StatusOK, w.Code)
	mockMenu.AssertExpectations(t)
}

func TestGetMenu_DBError(t *testing.T) {
	mockMenu := new(mocks.MockMenuRepository)
	mockMenu.On("GetByRestaurant", "rest-1").Return(nil, errors.New("db error"))

	r := setupRouter()
	r.GET("/menus/:id", handlers.GetMenu(mockMenu))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newJSONRequest("GET", "/menus/rest-1", nil))

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	mockMenu.AssertExpectations(t)
}

// ── CreateMenu ────────────────────────────────────────────────────────────────

func TestCreateMenu_OK(t *testing.T) {
	mockMenuRepo := new(mocks.MockMenuRepository)
	mockRestRepo := new(mocks.MockRestaurantRepository)

	// Mock Elasticsearch
	oldClient := elasticsearch.ESClient
	elasticsearch.ESClient = createMockESClient(`{"result":"created"}`, http.StatusCreated)
	defer func() { elasticsearch.ESClient = oldClient }()

	// Body correcto que espera el handler
	body := gin.H{
		"id_restaurante": "r-123",
		"platos": []gin.H{
			{
				"nombre":      "Casado",
				"descripcion": "Plato tradicional",
				"precio":      3500,
				"categoria":   "Almuerzos",
			},
		},
	}

	// Mock restaurante
	mockRestRepo.On("GetByID", "r-123").
		Return(&models.Restaurant{
			ID:              "r-123",
			IdAdministrador: "user-123",
		}, nil)

	// Mock create
	mockMenuRepo.On(
		"Create",
		"r-123",
		"Casado",
		"Plato tradicional",
		3500.0,
		"Almuerzos",
	).Return("p-123", nil)

	r := setupRouter()

	r.POST("/menu", func(c *gin.Context) {
		c.Set("user_id", "user-123")
		handlers.CreateMenu(mockMenuRepo, mockRestRepo)(c)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newJSONRequest("POST", "/menu", body))

	assert.Equal(t, http.StatusCreated, w.Code)

	mockMenuRepo.AssertExpectations(t)
	mockRestRepo.AssertExpectations(t)
}

func TestCreateMenu_RestaurantNotFound(t *testing.T) {
	mockMenu := new(mocks.MockMenuRepository)
	mockRest := new(mocks.MockRestaurantRepository)
	mockRest.On("GetByID", "rest-999").Return(nil, errors.New("not found"))

	r := setupRouter()
	r.POST("/menus", func(c *gin.Context) {
		c.Set("user_id", "user-123")
		handlers.CreateMenu(mockMenu, mockRest)(c)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newJSONRequest("POST", "/menus", map[string]interface{}{
		"id_restaurante": "rest-999",
		"platos": []map[string]interface{}{
			{"nombre": "Pizza", "precio": 5000.0},
		},
	}))

	assert.Equal(t, http.StatusNotFound, w.Code)
	mockMenu.AssertNotCalled(t, "Create")
}

func TestCreateMenu_NotAdmin(t *testing.T) {
	mockMenu := new(mocks.MockMenuRepository)
	mockRest := new(mocks.MockRestaurantRepository)
	mockRest.On("GetByID", "rest-1").Return(&models.Restaurant{
		ID: "rest-1", IdAdministrador: "otro-user",
	}, nil)

	r := setupRouter()
	r.POST("/menus", func(c *gin.Context) {
		c.Set("user_id", "user-123") // no es el admin del restaurante
		handlers.CreateMenu(mockMenu, mockRest)(c)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newJSONRequest("POST", "/menus", map[string]interface{}{
		"id_restaurante": "rest-1",
		"platos": []map[string]interface{}{
			{"nombre": "Pizza", "precio": 5000.0},
		},
	}))

	assert.Equal(t, http.StatusForbidden, w.Code)
	mockMenu.AssertNotCalled(t, "Create")
}

func TestCreateMenu_BadBody(t *testing.T) {
	mockMenu := new(mocks.MockMenuRepository)
	mockRest := new(mocks.MockRestaurantRepository)

	r := setupRouter()
	r.POST("/menus", func(c *gin.Context) {
		c.Set("user_id", "user-123")
		handlers.CreateMenu(mockMenu, mockRest)(c)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newJSONRequest("POST", "/menus", map[string]string{}))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ── UpdateMenu ────────────────────────────────────────────────────────────────

func TestUpdateMenu_OK(t *testing.T) {
	mockMenuRepo := new(mocks.MockMenuRepository)

	oldClient := elasticsearch.ESClient
	elasticsearch.ESClient = createMockESClient(`{"result":"updated"}`, http.StatusOK)
	defer func() { elasticsearch.ESClient = oldClient }()

	plato := models.MenuItem{
		ID:             "p-123",
		Nombre:         "Casado Especial",
		Precio:         4000,
		EstaDisponible: true,
	}

	// Mock autorización
	mockMenuRepo.On("GetAdminID", "p-123").
		Return("user-123", nil)

	// Mock update con los 6 parámetros reales
	mockMenuRepo.On(
		"Update",
		"p-123",
		"Casado Especial",
		"",
		4000.0,
		true,
		"",
	).Return(nil)

	r := setupRouter()

	r.PUT("/menu/:id", func(c *gin.Context) {
		c.Set("user_id", "user-123")
		handlers.UpdateMenu(mockMenuRepo)(c)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newJSONRequest("PUT", "/menu/p-123", plato))

	assert.Equal(t, http.StatusOK, w.Code)

	mockMenuRepo.AssertExpectations(t)
}

func TestUpdateMenu_NotAuthorized(t *testing.T) {
	mockMenu := new(mocks.MockMenuRepository)
	mockMenu.On("GetAdminID", "plato-1").Return("otro-user", nil)

	r := setupRouter()
	r.PUT("/menus/:id", func(c *gin.Context) {
		c.Set("user_id", "user-123")
		handlers.UpdateMenu(mockMenu)(c)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newJSONRequest("PUT", "/menus/plato-1", map[string]interface{}{
		"nombre": "Hack",
	}))

	assert.Equal(t, http.StatusForbidden, w.Code)
	mockMenu.AssertNotCalled(t, "Update")
}

func TestUpdateMenu_DBError(t *testing.T) {
	mockMenu := new(mocks.MockMenuRepository)
	mockMenu.On("GetAdminID", "plato-1").Return("user-123", nil)
	mockMenu.On("Update", "plato-1", "", "", 0.0, false, "").Return(errors.New("db error"))

	r := setupRouter()
	r.PUT("/menus/:id", func(c *gin.Context) {
		c.Set("user_id", "user-123")
		handlers.UpdateMenu(mockMenu)(c)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newJSONRequest("PUT", "/menus/plato-1", map[string]interface{}{}))

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	mockMenu.AssertExpectations(t)
}

// ── DeleteMenu ────────────────────────────────────────────────────────────────

func TestDeleteMenu_OK(t *testing.T) {
	mockMenuRepo := new(mocks.MockMenuRepository)

	// MOCK ELASTICSEARCH
	oldClient := elasticsearch.ESClient
	elasticsearch.ESClient = createMockESClient(`{"result":"deleted"}`, http.StatusOK)
	defer func() { elasticsearch.ESClient = oldClient }()

	mockMenuRepo.On("GetAdminID", "p-123").
		Return("user-123", nil)

	mockMenuRepo.On("Delete", "p-123").
		Return(nil)

	r := setupRouter()

	r.DELETE("/menu/:id", func(c *gin.Context) {
		c.Set("user_id", "user-123")
		handlers.DeleteMenu(mockMenuRepo)(c)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newJSONRequest("DELETE", "/menu/p-123", nil))

	assert.Equal(t, http.StatusOK, w.Code)

	mockMenuRepo.AssertExpectations(t)
}

func TestDeleteMenu_NotAuthorized(t *testing.T) {
	mockMenu := new(mocks.MockMenuRepository)
	mockMenu.On("GetAdminID", "plato-1").Return("otro-user", nil)

	r := setupRouter()
	r.DELETE("/menus/:id", func(c *gin.Context) {
		c.Set("user_id", "user-123")
		handlers.DeleteMenu(mockMenu)(c)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newJSONRequest("DELETE", "/menus/plato-1", nil))

	assert.Equal(t, http.StatusForbidden, w.Code)
	mockMenu.AssertNotCalled(t, "Delete")
}

func TestDeleteMenu_DBError(t *testing.T) {
	mockMenu := new(mocks.MockMenuRepository)
	mockMenu.On("GetAdminID", "plato-1").Return("user-123", nil)
	mockMenu.On("Delete", "plato-1").Return(errors.New("db error"))

	r := setupRouter()
	r.DELETE("/menus/:id", func(c *gin.Context) {
		c.Set("user_id", "user-123")
		handlers.DeleteMenu(mockMenu)(c)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newJSONRequest("DELETE", "/menus/plato-1", nil))

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	mockMenu.AssertExpectations(t)
}

// ── CreateReservation ─────────────────────────────────────────────────────────

func TestCreateReservation_OK(t *testing.T) {
	mockRes := new(mocks.MockReservationRepository)
	fecha := time.Now().Add(24 * time.Hour)

	mockRes.On("GetRestaurantCapacity", "rest-1").Return(50, nil)
	mockRes.On("Create", "user-123", "rest-1", mock.MatchedBy(func(t time.Time) bool {
		return t.Equal(fecha)
	}), 4).Return(&models.Reservation{
		ID:               "res-1",
		IdUsuario:        "user-123",
		IdRestaurante:    "rest-1",
		CantidadPersonas: 4,
		NombreEstado:     "pendiente",
	}, nil)

	r := setupRouter()
	r.POST("/reservations", func(c *gin.Context) {
		c.Set("user_id", "user-123")
		handlers.CreateReservation(mockRes)(c)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newJSONRequest("POST", "/reservations", map[string]interface{}{
		"id_restaurante":    "rest-1",
		"fecha_reserva":     fecha,
		"cantidad_personas": 4,
	}))

	assert.Equal(t, http.StatusCreated, w.Code)
	mockRes.AssertExpectations(t)
}

func TestCreateReservation_ExceedsCapacity(t *testing.T) {
	mockRes := new(mocks.MockReservationRepository)
	fecha := time.Now().Add(24 * time.Hour)
	mockRes.On("GetRestaurantCapacity", "rest-1").Return(5, nil)

	r := setupRouter()
	r.POST("/reservations", func(c *gin.Context) {
		c.Set("user_id", "user-123")
		handlers.CreateReservation(mockRes)(c)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newJSONRequest("POST", "/reservations", map[string]interface{}{
		"id_restaurante":    "rest-1",
		"fecha_reserva":     fecha,
		"cantidad_personas": 10,
	}))

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "capacidad")
	mockRes.AssertNotCalled(t, "Create")
}

func TestCreateReservation_RestaurantNotFound(t *testing.T) {
	mockRes := new(mocks.MockReservationRepository)
	fecha := time.Now().Add(24 * time.Hour)
	mockRes.On("GetRestaurantCapacity", "rest-999").Return(0, errors.New("not found"))

	r := setupRouter()
	r.POST("/reservations", func(c *gin.Context) {
		c.Set("user_id", "user-123")
		handlers.CreateReservation(mockRes)(c)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newJSONRequest("POST", "/reservations", map[string]interface{}{
		"id_restaurante":    "rest-999",
		"fecha_reserva":     fecha,
		"cantidad_personas": 2,
	}))

	assert.Equal(t, http.StatusNotFound, w.Code)
	mockRes.AssertNotCalled(t, "Create")
}

func TestCreateReservation_BadBody(t *testing.T) {
	mockRes := new(mocks.MockReservationRepository)

	r := setupRouter()
	r.POST("/reservations", func(c *gin.Context) {
		c.Set("user_id", "user-123")
		handlers.CreateReservation(mockRes)(c)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newJSONRequest("POST", "/reservations", map[string]string{}))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ── CancelReservation ─────────────────────────────────────────────────────────

func TestCancelReservation_OK(t *testing.T) {
	mockRes := new(mocks.MockReservationRepository)
	mockRes.On("GetOwnerID", "res-1").Return("user-123", nil)
	mockRes.On("Cancel", "res-1").Return(nil)

	r := setupRouter()
	r.DELETE("/reservations/:id", func(c *gin.Context) {
		c.Set("user_id", "user-123")
		handlers.CancelReservation(mockRes)(c)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newJSONRequest("DELETE", "/reservations/res-1", nil))

	assert.Equal(t, http.StatusOK, w.Code)
	mockRes.AssertExpectations(t)
}

func TestCancelReservation_NotAuthorized(t *testing.T) {
	mockRes := new(mocks.MockReservationRepository)
	mockRes.On("GetOwnerID", "res-1").Return("otro-user", nil)

	r := setupRouter()
	r.DELETE("/reservations/:id", func(c *gin.Context) {
		c.Set("user_id", "user-123")
		handlers.CancelReservation(mockRes)(c)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newJSONRequest("DELETE", "/reservations/res-1", nil))

	assert.Equal(t, http.StatusForbidden, w.Code)
	mockRes.AssertNotCalled(t, "Cancel")
}

// ── CreateOrder ───────────────────────────────────────────────────────────────

func TestCreateOrder_OK(t *testing.T) {
	mockOrder := new(mocks.MockOrderRepository)
	detalles := []models.OrderDetailRequest{
		{IdPlato: "plato-1", Cantidad: 2},
	}
	mockOrder.On("Create", "user-123", "rest-1", false, detalles).Return(&models.Order{
		ID:    "order-1",
		Total: 10000,
	}, nil)

	r := setupRouter()
	r.POST("/orders", func(c *gin.Context) {
		c.Set("user_id", "user-123")
		handlers.CreateOrder(mockOrder)(c)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newJSONRequest("POST", "/orders", map[string]interface{}{
		"id_restaurante": "rest-1",
		"es_recogida":    false,
		"detalles": []map[string]interface{}{
			{"id_plato": "plato-1", "cantidad": 2},
		},
	}))

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Contains(t, w.Body.String(), "order-1")
	mockOrder.AssertExpectations(t)
}

func TestCreateOrder_BadBody(t *testing.T) {
	mockOrder := new(mocks.MockOrderRepository)

	r := setupRouter()
	r.POST("/orders", func(c *gin.Context) {
		c.Set("user_id", "user-123")
		handlers.CreateOrder(mockOrder)(c)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newJSONRequest("POST", "/orders", map[string]string{}))

	assert.Equal(t, http.StatusBadRequest, w.Code)
	mockOrder.AssertNotCalled(t, "Create")
}

func TestCreateOrder_DBError(t *testing.T) {
	mockOrder := new(mocks.MockOrderRepository)
	detalles := []models.OrderDetailRequest{
		{IdPlato: "plato-1", Cantidad: 1},
	}
	mockOrder.On("Create", "user-123", "rest-1", false, detalles).
		Return(nil, errors.New("db error"))

	r := setupRouter()
	r.POST("/orders", func(c *gin.Context) {
		c.Set("user_id", "user-123")
		handlers.CreateOrder(mockOrder)(c)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newJSONRequest("POST", "/orders", map[string]interface{}{
		"id_restaurante": "rest-1",
		"detalles": []map[string]interface{}{
			{"id_plato": "plato-1", "cantidad": 1},
		},
	}))

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	mockOrder.AssertExpectations(t)
}

// ── GetOrder ──────────────────────────────────────────────────────────────────

func TestGetOrder_OK(t *testing.T) {
	mockOrder := new(mocks.MockOrderRepository)
	mockOrder.On("GetByID", "order-1").Return(&models.Order{
		ID:        "order-1",
		IdUsuario: "user-123",
		Total:     10000,
	}, nil)

	r := setupRouter()
	r.GET("/orders/:id", func(c *gin.Context) {
		c.Set("user_id", "user-123")
		handlers.GetOrder(mockOrder)(c)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newJSONRequest("GET", "/orders/order-1", nil))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "order-1")
	mockOrder.AssertExpectations(t)
}

func TestGetOrder_NotAuthorized(t *testing.T) {
	mockOrder := new(mocks.MockOrderRepository)
	mockOrder.On("GetByID", "order-1").Return(&models.Order{
		ID:        "order-1",
		IdUsuario: "otro-user",
	}, nil)

	r := setupRouter()
	r.GET("/orders/:id", func(c *gin.Context) {
		c.Set("user_id", "user-123")
		handlers.GetOrder(mockOrder)(c)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newJSONRequest("GET", "/orders/order-1", nil))

	assert.Equal(t, http.StatusForbidden, w.Code)
	mockOrder.AssertExpectations(t)
}

func TestGetOrder_NotFound(t *testing.T) {
	mockOrder := new(mocks.MockOrderRepository)
	mockOrder.On("GetByID", "order-999").Return(nil, errors.New("not found"))

	r := setupRouter()
	r.GET("/orders/:id", func(c *gin.Context) {
		c.Set("user_id", "user-123")
		handlers.GetOrder(mockOrder)(c)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newJSONRequest("GET", "/orders/order-999", nil))

	assert.Equal(t, http.StatusNotFound, w.Code)
	mockOrder.AssertExpectations(t)
}
