package handlers

import (
	"net/http"

	"restaurante/models"
	"restaurante/repository"

	"github.com/gin-gonic/gin"
)

func CreateOrder(orders repository.OrderRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		var req struct {
			IdRestaurante string                      `json:"id_restaurante" binding:"required"`
			EsRecogida    bool                        `json:"es_recogida"`
			Detalles      []models.OrderDetailRequest `json:"detalles"       binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			respondError(c, http.StatusBadRequest, err.Error())
			return
		}
		order, err := orders.Create(userID, req.IdRestaurante, req.EsRecogida, req.Detalles)
		if err != nil {
			respondError(c, http.StatusInternalServerError, "Error creando pedido")
			return
		}
		respondJSON(c, http.StatusCreated, order)
	}
}

func GetOrder(orders repository.OrderRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		order, err := orders.GetByID(c.Param("id"))
		if err != nil {
			respondError(c, http.StatusNotFound, "Orden no encontrada")
			return
		}
		if order.IdUsuario != userID {
			respondError(c, http.StatusForbidden, "No autorizado")
			return
		}
		respondJSON(c, http.StatusOK, order)
	}
}
