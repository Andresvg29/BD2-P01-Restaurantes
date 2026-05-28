package handlers

import (
	"log"
	"net/http"

	"restaurante/elasticsearch"
	"restaurante/models"
	"restaurante/repository"

	"github.com/gin-gonic/gin"
)

func GetMenu(menus repository.MenuRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		restaurantID := c.Param("id")
		items, err := menus.GetByRestaurant(restaurantID)
		if err != nil {
			respondError(c, http.StatusInternalServerError, "Error obteniendo menú")
			return
		}
		if items == nil {
			items = []models.MenuItem{}
		}
		respondJSON(c, http.StatusOK, gin.H{"id_restaurante": restaurantID, "platos": items})
	}
}

func CreateMenu(menus repository.MenuRepository, restaurants repository.RestaurantRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			IdRestaurante string `json:"id_restaurante" binding:"required"`
			Platos        []struct {
				Nombre      string  `json:"nombre"      binding:"required"`
				Descripcion string  `json:"descripcion"`
				Precio      float64 `json:"precio"      binding:"required,gt=0"`
				Categoria   string  `json:"categoria"`
			} `json:"platos" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			respondError(c, http.StatusBadRequest, err.Error())
			return
		}
		rest, err := restaurants.GetByID(req.IdRestaurante)
		if err != nil {
			respondError(c, http.StatusNotFound, "Restaurante no encontrado")
			return
		}
		if !requireAdminOfRestaurant(c, rest) {
			return
		}

		var created []models.MenuItem
		for _, p := range req.Platos {
			id, _ := menus.Create(req.IdRestaurante, p.Nombre, p.Descripcion, p.Precio, p.Categoria)
			item := models.MenuItem{
				ID:             id,
				IdRestaurante:  req.IdRestaurante,
				Nombre:         p.Nombre,
				Descripcion:    p.Descripcion,
				Precio:         p.Precio,
				EstaDisponible: true,
				Categoria:      p.Categoria,
			}
			if err := elasticsearch.IndexPlato(item); err != nil {
				log.Printf("Error indexando plato %s en Elasticsearch: %v", id, err)
			}
			created = append(created, item)
		}
		respondJSON(c, http.StatusCreated, created)
	}
}

func UpdateMenu(menus repository.MenuRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		platoID := c.Param("id")
		var req struct {
			Nombre         string  `json:"nombre"`
			Descripcion    string  `json:"descripcion"`
			Precio         float64 `json:"precio"`
			EstaDisponible bool    `json:"esta_disponible"`
			Categoria      string  `json:"categoria"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			respondError(c, http.StatusBadRequest, err.Error())
			return
		}
		if !requireAdminOfMenuItem(c, menus, platoID) {
			return
		}
		err := menus.Update(platoID, req.Nombre, req.Descripcion, req.Precio, req.EstaDisponible, req.Categoria)
		if err != nil {
			respondError(c, http.StatusInternalServerError, "Error actualizando plato")
			return
		}
		updatedItem := models.MenuItem{
			ID:             platoID,
			Nombre:         req.Nombre,
			Descripcion:    req.Descripcion,
			Precio:         req.Precio,
			EstaDisponible: req.EstaDisponible,
			Categoria:      req.Categoria,
		}
		if err := elasticsearch.IndexPlato(updatedItem); err != nil {
			log.Printf("Error actualizando plato %s en Elasticsearch: %v", platoID, err)
		}
		respondJSON(c, http.StatusOK, gin.H{"message": "Actualizado"})
	}
}

func DeleteMenu(menus repository.MenuRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		platoID := c.Param("id")
		if !requireAdminOfMenuItem(c, menus, platoID) {
			return
		}
		if err := menus.Delete(platoID); err != nil {
			respondError(c, http.StatusInternalServerError, "Error eliminando plato")
			return
		}
		if err := elasticsearch.DeletePlato(platoID); err != nil {
			log.Printf("Error eliminando plato %s de Elasticsearch: %v", platoID, err)
		}
		respondJSON(c, http.StatusOK, gin.H{"message": "Eliminado"})
	}
}
