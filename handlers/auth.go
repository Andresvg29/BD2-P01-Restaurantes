package handlers

import (
	"context"
	"net/http"

	"restaurante/keycloak"
	"restaurante/repository"

	"github.com/gin-gonic/gin"
)

// Login autentica al usuario contra Keycloak y devuelve el JWT
func Login(kc keycloak.ClientInterface) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Email    string `json:"email"    binding:"required"`
			Password string `json:"password" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			respondError(c, http.StatusBadRequest, err.Error())
			return
		}

		token, err := kc.LoginUser(context.Background(), req.Email, req.Password)
		if err != nil {
			respondError(c, http.StatusUnauthorized, "Credenciales invalidas")
			return
		}

		respondJSON(c, http.StatusOK, gin.H{
			"access_token":  token.AccessToken,
			"refresh_token": token.RefreshToken,
			"expires_in":    token.ExpiresIn,
			"token_type":    token.TokenType,
		})
	}
}

// crea el usuario en Keycloak y guarda referencia en la BD
func RegisterUser(kc keycloak.ClientInterface, users repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Nombre   string `json:"nombre"   binding:"required"`
			Apellido string `json:"apellido" binding:"required"`
			Email    string `json:"email"    binding:"required"`
			Password string `json:"password" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			respondError(c, http.StatusBadRequest, err.Error())
			return
		}
		// crear en Keycloak
		keycloakID, err := kc.RegisterUser(context.Background(), req.Nombre, req.Apellido, req.Email, req.Password)
		if err != nil {
			respondError(c, http.StatusConflict, err.Error())
			return
		}

		// sincronizar en la BD local
		if err := users.UpsertFromToken(keycloakID, req.Nombre, req.Email, false); err != nil {
			// el usuario existe en Keycloak aunque falle la BD local
			respondJSON(c, http.StatusCreated, gin.H{
				"id":      keycloakID,
				"message": "Usuario creado (sincronizacion BD pendiente)",
			})
			return
		}

		respondJSON(c, http.StatusCreated, gin.H{
			"id":      keycloakID,
			"nombre":  req.Nombre,
			"email":   req.Email,
			"message": "Usuario registrado exitosamente",
		})
	}
}


