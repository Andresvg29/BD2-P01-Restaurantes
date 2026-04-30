package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"

	"github.com/andre/restaurante-api/repository"
	"github.com/gin-gonic/gin"
)

// devuelve instrucciones para obtener un token de Keycloak
func LoginInfo() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "Para obtener token, realiza una solicitud POST a:",
			"url": fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token",
				os.Getenv("KEYCLOAK_URL"), os.Getenv("KEYCLOAK_REALM")),
			"body": gin.H{
				"grant_type":    "password",
				"client_id":     os.Getenv("KEYCLOAK_CLIENT_ID"),
				"client_secret": os.Getenv("KEYCLOAK_CLIENT_SECRET"),
				"username":      "cliente-prueba",
				"password":      "1234",
			},
		})
	}
}

// crea un usuario nuevo en la BD
func RegisterUser(users repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Nombre   string `json:"nombre"   binding:"required"`
			Email    string `json:"email"    binding:"required"`
			Password string `json:"password" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		// Verificar si el email ya existe
		if _, err := users.GetByEmail(req.Email); err == nil {
			c.JSON(409, gin.H{"error": "El email ya está registrado"})
			return
		}

		id, err := users.Create(req.Nombre, req.Email, req.Password)
		if err != nil {
			c.JSON(500, gin.H{"error": "Error registrando usuario"})
			return
		}

		c.JSON(201, gin.H{
			"id":      id,
			"nombre":  req.Nombre,
			"email":   req.Email,
			"message": "Usuario registrado exitosamente",
		})
	}
}

// devuelve el perfil del usuario autenticado
func GetCurrentUser(users repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		user, err := users.GetByID(userID)
		if err != nil {
			c.JSON(404, gin.H{"error": "Usuario no encontrado"})
			return
		}
		c.JSON(200, user)
	}
}

//actualiza nombre y/o email del usuario
func UpdateUser(users repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		if userID != c.Param("id") {
			c.JSON(403, gin.H{"error": "No autorizado"})
			return
		}
		var req struct {
			Nombre string `json:"nombre"`
			Email  string `json:"email"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		if err := users.Update(userID, req.Nombre, req.Email); err != nil {
			c.JSON(500, gin.H{"error": "Error actualizando usuario"})
			return
		}
		c.JSON(200, gin.H{"message": "Usuario actualizado"})
	}
}

//elimina al usuario autenticado
func DeleteUser(users repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		if userID != c.Param("id") {
			c.JSON(403, gin.H{"error": "No autorizado"})
			return
		}
		if err := users.Delete(userID); err != nil {
			c.JSON(500, gin.H{"error": "Error eliminando usuario"})
			return
		}
		c.JSON(200, gin.H{"message": "Usuario eliminado"})
	}
}

// isSQLNoRows es un helper para distinguir "no encontrado" de error real
func isSQLNoRows(err error) bool {
	return err == sql.ErrNoRows
}

// var _ = isSQLNoRows // evitar "declared and not used" si no se usa en algun repositorio
var _ = http.StatusOK