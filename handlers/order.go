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
