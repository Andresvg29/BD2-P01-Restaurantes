package handlers

import (
	"log"
	"net/http"
	"time"

	"restaurante/elasticsearch"
	"restaurante/models"
	"restaurante/repository"

	"github.com/gin-gonic/gin"
)

func ListRestaurants(restaurants repository.RestaurantRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		list, err := restaurants.List()
		if err != nil {
			c.JSON(500, gin.H{"error": "Error obteniendo restaurantes"})
			return
		}
		if list == nil {
			list = []models.Restaurant{}
		}
		c.JSON(200, list)
	}
}

func CreateRestaurant(restaurants repository.RestaurantRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		var req struct {
			Nombre         string `json:"nombre"          binding:"required"`
			Direccion      string `json:"direccion"       binding:"required"`
			Telefono       string `json:"telefono"`
			CapacidadTotal int    `json:"capacidad_total" binding:"required,gt=0"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		id, err := restaurants.Create(req.Nombre, req.Direccion, req.Telefono, userID, req.CapacidadTotal)
		if err != nil {
			c.JSON(500, gin.H{"error": "Error creando restaurante"})
			return
		}
		c.JSON(201, models.Restaurant{
			ID: id, Nombre: req.Nombre, Direccion: req.Direccion,
			Telefono: req.Telefono, IdAdministrador: userID, CapacidadTotal: req.CapacidadTotal,
		})
	}
}

func GetMenu(menus repository.MenuRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		restaurantID := c.Param("id")
		items, err := menus.GetByRestaurant(restaurantID)
		if err != nil {
			c.JSON(500, gin.H{"error": "Error obteniendo menú"})
			return
		}
		if items == nil {
			items = []models.MenuItem{}
		}
		c.JSON(200, gin.H{"id_restaurante": restaurantID, "platos": items})
	}
}

func CreateMenu(menus repository.MenuRepository, restaurants repository.RestaurantRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		var req struct {
			IdRestaurante string `json:"id_restaurante" binding:"required"`
			Platos        []struct {
				Nombre      string  `json:"nombre"      binding:"required"`
				Descripcion string  `json:"descripcion"`
				Precio      float64 `json:"precio"      binding:"required,gt=0"`
				Categoria   string  `json:"categoria"`
			} `json:"platos" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		// Verificar que el usuario es el admin del restaurante
		rest, err := restaurants.GetByID(req.IdRestaurante)
		if err != nil {
			c.JSON(404, gin.H{"error": "Restaurante no encontrado"})
			return
		}
		if rest.IdAdministrador != userID {
			c.JSON(403, gin.H{"error": "No autorizado"})
			return
		}

		var created []models.MenuItem
		for _, p := range req.Platos {
			id, _ := menus.Create(req.IdRestaurante, p.Nombre, p.Descripcion, p.Precio, p.Categoria)
			item := models.MenuItem{
				ID:             id,
				IdRestaurante:  req.IdRestaurante,
				Nombre:         p.Nombre,
				Descripcion:    p.Descripcion,
				Precio:         p.Precio,
				EstaDisponible: true,
				Categoria:      p.Categoria,
			}
			if err := elasticsearch.IndexPlato(item); err != nil {
				log.Printf("Error indexando plato %s en Elasticsearch: %v", id, err)
			}
			created = append(created, item)
		}
		c.JSON(201, created)
	}
}

func UpdateMenu(menus repository.MenuRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		platoID := c.Param("id")
		var req struct {
			Nombre         string  `json:"nombre"`
			Descripcion    string  `json:"descripcion"`
			Precio         float64 `json:"precio"`
			EstaDisponible bool    `json:"esta_disponible"`
			Categoria      string  `json:"categoria"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		adminID, err := menus.GetAdminID(platoID)
		if err != nil || adminID != userID {
			c.JSON(403, gin.H{"error": "No autorizado"})
			return
		}
		err = menus.Update(platoID, req.Nombre, req.Descripcion, req.Precio, req.EstaDisponible, req.Categoria)
		if err != nil {
			c.JSON(500, gin.H{"error": "Error actualizando plato"})
			return
		}
		updatedItem := models.MenuItem{
			ID:             platoID,
			Nombre:         req.Nombre,
			Descripcion:    req.Descripcion,
			Precio:         req.Precio,
			EstaDisponible: req.EstaDisponible,
			Categoria:      req.Categoria,
		}
		if err := elasticsearch.IndexPlato(updatedItem); err != nil {
			log.Printf("Error actualizando plato %s en Elasticsearch: %v", platoID, err)
		}
		c.JSON(200, gin.H{"message": "Actualizado"})
	}
}

func DeleteMenu(menus repository.MenuRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		platoID := c.Param("id")
		adminID, err := menus.GetAdminID(platoID)
		if err != nil || adminID != userID {
			c.JSON(403, gin.H{"error": "No autorizado"})
			return
		}
		if err := menus.Delete(platoID); err != nil {
			c.JSON(500, gin.H{"error": "Error eliminando plato"})
			return
		}
		if err := elasticsearch.DeletePlato(platoID); err != nil {
			log.Printf("Error eliminando plato %s de Elasticsearch: %v", platoID, err)
		}
		c.JSON(200, gin.H{"message": "Eliminado"})
	}
}

func CreateReservation(reservations repository.ReservationRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		var req struct {
			IdRestaurante    string    `json:"id_restaurante"    binding:"required"`
			FechaReserva     time.Time `json:"fecha_reserva"     binding:"required"`
			CantidadPersonas int       `json:"cantidad_personas" binding:"required,gt=0"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		cap, err := reservations.GetRestaurantCapacity(req.IdRestaurante)
		if err != nil {
			c.JSON(404, gin.H{"error": "Restaurante no encontrado"})
			return
		}
		if req.CantidadPersonas > cap {
			c.JSON(400, gin.H{"error": "Excede capacidad"})
			return
		}
		res, err := reservations.Create(userID, req.IdRestaurante, req.FechaReserva, req.CantidadPersonas)
		if err != nil {
			c.JSON(500, gin.H{"error": "Error creando reserva"})
			return
		}
		c.JSON(201, res)
	}
}

func CancelReservation(reservations repository.ReservationRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		reservationID := c.Param("id")
		ownerID, err := reservations.GetOwnerID(reservationID)
		if err != nil || ownerID != userID {
			c.JSON(403, gin.H{"error": "No autorizado"})
			return
		}
		reservations.Cancel(reservationID)
		c.JSON(200, gin.H{"message": "Cancelada"})
	}
}

func CreateOrder(orders repository.OrderRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		var req struct {
			IdRestaurante string                      `json:"id_restaurante" binding:"required"`
			EsRecogida    bool                        `json:"es_recogida"`
			Detalles      []models.OrderDetailRequest `json:"detalles"       binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		order, err := orders.Create(userID, req.IdRestaurante, req.EsRecogida, req.Detalles)
		if err != nil {
			c.JSON(500, gin.H{"error": "Error creando pedido"})
			return
		}
		c.JSON(201, order)
	}
}

func GetOrder(orders repository.OrderRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		order, err := orders.GetByID(c.Param("id"))
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Orden no encontrada"})
			return
		}
		if order.IdUsuario != userID {
			c.JSON(http.StatusForbidden, gin.H{"error": "No autorizado"})
			return
		}
		c.JSON(200, order)
	}
}
