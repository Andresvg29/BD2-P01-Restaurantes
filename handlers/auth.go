package handlers

import (
	"context"
	"net/http"

	"restaurante/keycloak"
	"restaurante/repository"

	"github.com/gin-gonic/gin"
)

// Login autentica al usuario contra Keycloak y devuelve el JWT
func Login(kc *keycloak.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Email    string `json:"email"    binding:"required"`
			Password string `json:"password" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		token, err := kc.LoginUser(context.Background(), req.Email, req.Password)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Credenciales invalidas"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"access_token":  token.AccessToken,
			"refresh_token": token.RefreshToken,
			"expires_in":    token.ExpiresIn,
			"token_type":    token.TokenType,
		})
	}
}

// crea el usuario en Keycloak y guarda referencia en la BD
func RegisterUser(kc *keycloak.Client, users repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Nombre   string `json:"nombre"   binding:"required"`
			Apellido string `json:"apellido" binding:"required"`
			Email    string `json:"email"    binding:"required"`
			Password string `json:"password" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		// crear en Keycloak
		keycloakID, err := kc.RegisterUser(context.Background(), req.Nombre, req.Apellido, req.Email, req.Password)
		if err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}

		// sincronizar en la BD local
		if err := users.UpsertFromToken(keycloakID, req.Nombre, req.Email, false); err != nil {
			// el usuario existe en Keycloak aunque falle la BD local
			c.JSON(http.StatusCreated, gin.H{
				"id":      keycloakID,
				"message": "Usuario creado (sincronizacion BD pendiente)",
			})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"id":      keycloakID,
			"nombre":  req.Nombre,
			"email":   req.Email,
			"message": "Usuario registrado exitosamente",
		})
	}
}


