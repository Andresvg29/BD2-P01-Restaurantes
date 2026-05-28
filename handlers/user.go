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
			respondError(c, http.StatusNotFound, "Usuario no encontrado")
			return
		}
		respondJSON(c, http.StatusOK, user)
	}
}

func UpdateUser(users repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		if !requireSameUser(c) {
			return
		}
		var req struct {
			Nombre string `json:"nombre"`
			Email  string `json:"email"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			respondError(c, http.StatusBadRequest, err.Error())
			return
		}
		if err := users.Update(userID, req.Nombre, req.Email); err != nil {
			respondError(c, http.StatusInternalServerError, "Error actualizando usuario")
			return
		}
		respondJSON(c, http.StatusOK, gin.H{"message": "Usuario actualizado"})
	}
}

func DeleteUser(users repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		if !requireSameUser(c) {
			return
		}
		if err := users.Delete(userID); err != nil {
			respondError(c, http.StatusInternalServerError, "Error eliminando usuario")
			return
		}
		respondJSON(c, http.StatusOK, gin.H{"message": "Usuario eliminado"})
	}
}
