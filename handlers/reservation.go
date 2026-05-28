package handlers

import (
	"time"

	"restaurante/repository"

	"github.com/gin-gonic/gin"
)

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
