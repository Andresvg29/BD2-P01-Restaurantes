package handlers

import (
	"net/http"

	"restaurante/models"
	"restaurante/repository"

	"github.com/gin-gonic/gin"
)

func respondError(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"error": msg})
}

func respondJSON(c *gin.Context, status int, data interface{}) {
	c.JSON(status, data)
}

func requireSameUser(c *gin.Context) bool {
	if c.GetString("user_id") != c.Param("id") {
		respondError(c, http.StatusForbidden, "No autorizado")
		return false
	}
	return true
}

func requireAdminOfRestaurant(c *gin.Context, rest *models.Restaurant) bool {
	if rest.IdAdministrador != c.GetString("user_id") {
		respondError(c, http.StatusForbidden, "No autorizado")
		return false
	}
	return true
}

func requireAdminOfMenuItem(c *gin.Context, menus repository.MenuRepository, platoID string) bool {
	adminID, err := menus.GetAdminID(platoID)
	if err != nil || adminID != c.GetString("user_id") {
		respondError(c, http.StatusForbidden, "No autorizado")
		return false
	}
	return true
}

func requireOwnerOfReservation(c *gin.Context, reservations repository.ReservationRepository, resID string) bool {
	ownerID, err := reservations.GetOwnerID(resID)
	if err != nil || ownerID != c.GetString("user_id") {
		respondError(c, http.StatusForbidden, "No autorizado")
		return false
	}
	return true
}
