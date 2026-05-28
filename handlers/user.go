package handlers

import (
	"net/http"

	"restaurante/repository"

	"github.com/gin-gonic/gin"
)

func GetCurrentUser(users repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		user, err := users.GetByID(userID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Usuario no encontrado"})
			return
		}
		c.JSON(http.StatusOK, user)
	}
}

func UpdateUser(users repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		if userID != c.Param("id") {
			c.JSON(http.StatusForbidden, gin.H{"error": "No autorizado"})
			return
		}
		var req struct {
			Nombre string `json:"nombre"`
			Email  string `json:"email"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := users.Update(userID, req.Nombre, req.Email); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error actualizando usuario"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Usuario actualizado"})
	}
}

func DeleteUser(users repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		if userID != c.Param("id") {
			c.JSON(http.StatusForbidden, gin.H{"error": "No autorizado"})
			return
		}
		if err := users.Delete(userID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error eliminando usuario"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Usuario eliminado"})
	}
}
