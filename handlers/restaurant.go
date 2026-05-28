package handlers

import (
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


