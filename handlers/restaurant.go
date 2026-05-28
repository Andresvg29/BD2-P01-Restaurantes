package handlers

import (
	"net/http"

	"restaurante/models"
	"restaurante/repository"

	"github.com/gin-gonic/gin"
)

func ListRestaurants(restaurants repository.RestaurantRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		list, err := restaurants.List()
		if err != nil {
			respondError(c, http.StatusInternalServerError, "Error obteniendo restaurantes")
			return
		}
		if list == nil {
			list = []models.Restaurant{}
		}
		respondJSON(c, http.StatusOK, list)
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
			respondError(c, http.StatusBadRequest, err.Error())
			return
		}
		id, err := restaurants.Create(req.Nombre, req.Direccion, req.Telefono, userID, req.CapacidadTotal)
		if err != nil {
			respondError(c, http.StatusInternalServerError, "Error creando restaurante")
			return
		}
		respondJSON(c, http.StatusCreated, models.Restaurant{
			ID: id, Nombre: req.Nombre, Direccion: req.Direccion,
			Telefono: req.Telefono, IdAdministrador: userID, CapacidadTotal: req.CapacidadTotal,
		})
	}
}


