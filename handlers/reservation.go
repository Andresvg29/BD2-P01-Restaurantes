package handlers

import (
	"net/http"
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
			respondError(c, http.StatusBadRequest, err.Error())
			return
		}
		cap, err := reservations.GetRestaurantCapacity(req.IdRestaurante)
		if err != nil {
			respondError(c, http.StatusNotFound, "Restaurante no encontrado")
			return
		}
		if req.CantidadPersonas > cap {
			respondError(c, http.StatusBadRequest, "Excede capacidad")
			return
		}
		res, err := reservations.Create(userID, req.IdRestaurante, req.FechaReserva, req.CantidadPersonas)
		if err != nil {
			respondError(c, http.StatusInternalServerError, "Error creando reserva")
			return
		}
		respondJSON(c, http.StatusCreated, res)
	}
}

func CancelReservation(reservations repository.ReservationRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		reservationID := c.Param("id")
		if !requireOwnerOfReservation(c, reservations, reservationID) {
			return
		}
		if err := reservations.Cancel(reservationID); err != nil {
			respondError(c, http.StatusInternalServerError, "Error cancelando reserva")
			return
		}
		respondJSON(c, http.StatusOK, gin.H{"message": "Cancelada"})
	}
}
