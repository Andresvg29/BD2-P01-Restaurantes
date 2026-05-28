package mongo_test

import (
	"testing"
	"time"

	"restaurante/models"
	repoMongo "restaurante/repository/mongo"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
)

// ============================================================
// UserRepository — 15 tests
// ============================================================

func TestUserGetByID_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewUserRepository(mt.Client.Database("test"))
		now := time.Now()
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "test.usuarios", mtest.FirstBatch,
			bson.D{
				{Key: "_id", Value: "u-1"},
				{Key: "nombre", Value: "Ana"},
				{Key: "email", Value: "ana@mail.com"},
				{Key: "id_rol", Value: int32(2)},
				{Key: "fecha_creacion", Value: primitive.NewDateTimeFromTime(now)},
			},
		))
		user, err := repo.GetByID("u-1")
		assert.NoError(t, err)
		assert.Equal(t, "u-1", user.ID)
		assert.Equal(t, "Ana", user.Nombre)
		assert.Equal(t, "ana@mail.com", user.Email)
		assert.Equal(t, 2, user.IdRol)
		assert.WithinDuration(t, now, user.FechaCreacion, time.Second)
	})
}

func TestUserGetByID_NotFound(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewUserRepository(mt.Client.Database("test"))
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "test.usuarios", mtest.FirstBatch))
		user, err := repo.GetByID("u-99")
		assert.Error(t, err)
		assert.Equal(t, mongo.ErrNoDocuments, err)
		assert.Nil(t, user)
	})
}

func TestUserGetByEmail_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewUserRepository(mt.Client.Database("test"))
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "test.usuarios", mtest.FirstBatch,
			bson.D{
				{Key: "_id", Value: "u-1"},
				{Key: "email", Value: "ana@mail.com"},
			},
		))
		user, err := repo.GetByEmail("ana@mail.com")
		assert.NoError(t, err)
		assert.Equal(t, "u-1", user.ID)
		assert.Equal(t, "ana@mail.com", user.Email)
	})
}

func TestUserGetByEmail_NotFound(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewUserRepository(mt.Client.Database("test"))
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "test.usuarios", mtest.FirstBatch))
		user, err := repo.GetByEmail("no@mail.com")
		assert.Error(t, err)
		assert.Equal(t, mongo.ErrNoDocuments, err)
		assert.Nil(t, user)
	})
}

func TestUserCreate_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewUserRepository(mt.Client.Database("test"))
		mt.AddMockResponses(mtest.CreateSuccessResponse(bson.E{Key: "n", Value: 1}))
		id, err := repo.Create("Ana", "ana@mail.com", "kc-123")
		assert.NoError(t, err)
		assert.NotEmpty(t, id)
	})
}

func TestUserCreate_DBError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewUserRepository(mt.Client.Database("test"))
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    11000,
			Message: "duplicate key",
		}))
		id, err := repo.Create("Ana", "ana@mail.com", "kc-123")
		assert.Error(t, err)
		assert.Empty(t, id)
	})
}

func TestUserUpdate_Both(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewUserRepository(mt.Client.Database("test"))
		mt.AddMockResponses(bson.D{
			{Key: "ok", Value: 1},
			{Key: "nModified", Value: 1},
			{Key: "n", Value: 1},
		})
		err := repo.Update("u-1", "Ana Maria", "ana@nuevo.com")
		assert.NoError(t, err)
	})
}

func TestUserUpdate_OnlyNombre(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewUserRepository(mt.Client.Database("test"))
		mt.AddMockResponses(bson.D{
			{Key: "ok", Value: 1},
			{Key: "nModified", Value: 1},
			{Key: "n", Value: 1},
		})
		err := repo.Update("u-1", "Ana Maria", "")
		assert.NoError(t, err)
	})
}

func TestUserUpdate_OnlyEmail(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewUserRepository(mt.Client.Database("test"))
		mt.AddMockResponses(bson.D{
			{Key: "ok", Value: 1},
			{Key: "nModified", Value: 1},
			{Key: "n", Value: 1},
		})
		err := repo.Update("u-1", "", "ana@nuevo.com")
		assert.NoError(t, err)
	})
}

func TestUserUpdate_Empty(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewUserRepository(mt.Client.Database("test"))
		mt.AddMockResponses(bson.D{
			{Key: "ok", Value: 1},
			{Key: "nModified", Value: 0},
			{Key: "n", Value: 1},
		})
		err := repo.Update("u-1", "", "")
		assert.NoError(t, err)
	})
}

func TestUserUpdate_DBError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewUserRepository(mt.Client.Database("test"))
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    50,
			Message: "ns not found",
		}))
		err := repo.Update("u-1", "Ana", "")
		assert.Error(t, err)
	})
}

func TestUserDelete_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewUserRepository(mt.Client.Database("test"))
		mt.AddMockResponses(bson.D{
			{Key: "ok", Value: 1},
			{Key: "n", Value: 1},
		})
		err := repo.Delete("u-1")
		assert.NoError(t, err)
	})
}

func TestUserDelete_DBError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewUserRepository(mt.Client.Database("test"))
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    50,
			Message: "ns not found",
		}))
		err := repo.Delete("u-99")
		assert.Error(t, err)
	})
}

func TestUserUpsertFromToken_Cliente(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewUserRepository(mt.Client.Database("test"))
		mt.AddMockResponses(bson.D{
			{Key: "ok", Value: 1},
			{Key: "nModified", Value: 1},
			{Key: "n", Value: 1},
			{Key: "upserted", Value: bson.A{}},
		})
		err := repo.UpsertFromToken("u-1", "Ana", "ana@mail.com", false)
		assert.NoError(t, err)
	})
}

func TestUserUpsertFromToken_Admin(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewUserRepository(mt.Client.Database("test"))
		mt.AddMockResponses(bson.D{
			{Key: "ok", Value: 1},
			{Key: "nModified", Value: 1},
			{Key: "n", Value: 1},
			{Key: "upserted", Value: bson.A{}},
		})
		err := repo.UpsertFromToken("u-1", "Admin", "admin@mail.com", true)
		assert.NoError(t, err)
	})
}

// ============================================================
// RestaurantRepository — 7 tests
// ============================================================

func TestRestaurantList_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewRestaurantRepository(mt.Client.Database("test"), nil)
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "test.restaurantes", mtest.FirstBatch,
			bson.D{
				{Key: "_id", Value: "r-1"},
				{Key: "nombre", Value: "Rest A"},
				{Key: "direccion", Value: "Calle 1"},
				{Key: "telefono", Value: "123456789"},
				{Key: "id_administrador", Value: "u-1"},
				{Key: "capacidad_total", Value: int32(50)},
			},
			bson.D{
				{Key: "_id", Value: "r-2"},
				{Key: "nombre", Value: "Rest B"},
				{Key: "direccion", Value: "Calle 2"},
				{Key: "telefono", Value: "987654321"},
				{Key: "id_administrador", Value: "u-2"},
				{Key: "capacidad_total", Value: int32(100)},
			},
		))
		list, err := repo.List()
		assert.NoError(t, err)
		assert.Len(t, list, 2)
		assert.Equal(t, "Rest A", list[0].Nombre)
		assert.Equal(t, "Rest B", list[1].Nombre)
		assert.Equal(t, 50, list[0].CapacidadTotal)
		assert.Equal(t, 100, list[1].CapacidadTotal)
	})
}

func TestRestaurantList_Empty(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewRestaurantRepository(mt.Client.Database("test"), nil)
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "test.restaurantes", mtest.FirstBatch))
		list, err := repo.List()
		assert.NoError(t, err)
		assert.Empty(t, list)
	})
}

func TestRestaurantList_DBError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewRestaurantRepository(mt.Client.Database("test"), nil)
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    50,
			Message: "error",
		}))
		list, err := repo.List()
		assert.Error(t, err)
		assert.Nil(t, list)
	})
}

func TestRestaurantGetByID_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewRestaurantRepository(mt.Client.Database("test"), nil)
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "test.restaurantes", mtest.FirstBatch,
			bson.D{
				{Key: "_id", Value: "r-1"},
				{Key: "nombre", Value: "Rest A"},
				{Key: "direccion", Value: "Calle 1"},
				{Key: "telefono", Value: "123456789"},
				{Key: "id_administrador", Value: "u-1"},
				{Key: "capacidad_total", Value: int32(50)},
			},
		))
		rest, err := repo.GetByID("r-1")
		assert.NoError(t, err)
		assert.Equal(t, "r-1", rest.ID)
		assert.Equal(t, "Rest A", rest.Nombre)
		assert.Equal(t, 50, rest.CapacidadTotal)
	})
}

func TestRestaurantGetByID_NotFound(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewRestaurantRepository(mt.Client.Database("test"), nil)
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "test.restaurantes", mtest.FirstBatch))
		rest, err := repo.GetByID("r-99")
		assert.Error(t, err)
		assert.Equal(t, mongo.ErrNoDocuments, err)
		assert.Nil(t, rest)
	})
}

func TestRestaurantCreate_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewRestaurantRepository(mt.Client.Database("test"), nil)
		mt.AddMockResponses(mtest.CreateSuccessResponse(bson.E{Key: "n", Value: 1}))
		id, err := repo.Create("Rest A", "Calle 1", "123456789", "u-1", 50)
		assert.NoError(t, err)
		assert.NotEmpty(t, id)
	})
}

func TestRestaurantCreate_DBError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewRestaurantRepository(mt.Client.Database("test"), nil)
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    11000,
			Message: "duplicate key",
		}))
		id, err := repo.Create("Rest A", "Calle 1", "123456789", "u-1", 50)
		assert.Error(t, err)
		assert.Empty(t, id)
	})
}

// ============================================================
// MenuRepository — 18 tests
// ============================================================

func TestMenuList_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewMenuRepository(mt.Client.Database("test"))
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "test.platos", mtest.FirstBatch,
			bson.D{
				{Key: "_id", Value: "p-1"},
				{Key: "id_restaurante", Value: "r-1"},
				{Key: "nombre", Value: "Plato 1"},
				{Key: "descripcion", Value: "Desc 1"},
				{Key: "precio", Value: 10.5},
				{Key: "esta_disponible", Value: true},
				{Key: "categoria", Value: "Entrada"},
			},
			bson.D{
				{Key: "_id", Value: "p-2"},
				{Key: "id_restaurante", Value: "r-1"},
				{Key: "nombre", Value: "Plato 2"},
				{Key: "descripcion", Value: "Desc 2"},
				{Key: "precio", Value: 20.0},
				{Key: "esta_disponible", Value: false},
				{Key: "categoria", Value: "Principal"},
			},
		))
		items, err := repo.List()
		assert.NoError(t, err)
		assert.Len(t, items, 2)
		assert.Equal(t, "Plato 1", items[0].Nombre)
		assert.Equal(t, 10.5, items[0].Precio)
		assert.Equal(t, true, items[0].EstaDisponible)
	})
}

func TestMenuList_Empty(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewMenuRepository(mt.Client.Database("test"))
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "test.platos", mtest.FirstBatch))
		items, err := repo.List()
		assert.NoError(t, err)
		assert.Empty(t, items)
	})
}

func TestMenuList_DBError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewMenuRepository(mt.Client.Database("test"))
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    50,
			Message: "error",
		}))
		items, err := repo.List()
		assert.Error(t, err)
		assert.Nil(t, items)
	})
}

func TestMenuGetByRestaurant_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewMenuRepository(mt.Client.Database("test"))
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "test.platos", mtest.FirstBatch,
			bson.D{
				{Key: "_id", Value: "p-1"},
				{Key: "id_restaurante", Value: "r-1"},
				{Key: "nombre", Value: "Plato 1"},
				{Key: "descripcion", Value: "Desc 1"},
				{Key: "precio", Value: 10.5},
				{Key: "esta_disponible", Value: true},
				{Key: "categoria", Value: "Entrada"},
			},
		))
		items, err := repo.GetByRestaurant("r-1")
		assert.NoError(t, err)
		assert.Len(t, items, 1)
		assert.Equal(t, "Plato 1", items[0].Nombre)
	})
}

func TestMenuGetByRestaurant_Empty(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewMenuRepository(mt.Client.Database("test"))
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "test.platos", mtest.FirstBatch))
		items, err := repo.GetByRestaurant("r-99")
		assert.NoError(t, err)
		assert.Empty(t, items)
	})
}

func TestMenuGetByRestaurant_DBError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewMenuRepository(mt.Client.Database("test"))
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    50,
			Message: "error",
		}))
		items, err := repo.GetByRestaurant("r-99")
		assert.Error(t, err)
		assert.Nil(t, items)
	})
}

func TestMenuCreate_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewMenuRepository(mt.Client.Database("test"))
		mt.AddMockResponses(mtest.CreateSuccessResponse(bson.E{Key: "n", Value: 1}))
		id, err := repo.Create("r-1", "Plato 1", "Desc 1", 10.5, "Entrada")
		assert.NoError(t, err)
		assert.NotEmpty(t, id)
	})
}

func TestMenuCreate_DefaultDescription(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewMenuRepository(mt.Client.Database("test"))
		mt.AddMockResponses(mtest.CreateSuccessResponse(bson.E{Key: "n", Value: 1}))
		id, err := repo.Create("r-1", "Plato 1", "", 10.5, "Entrada")
		assert.NoError(t, err)
		assert.NotEmpty(t, id)
	})
}

func TestMenuCreate_DBError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewMenuRepository(mt.Client.Database("test"))
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    11000,
			Message: "duplicate key",
		}))
		_, err := repo.Create("r-1", "Plato 1", "Desc", 10.5, "Entrada")
		assert.Error(t, err)
	})
}

func TestMenuUpdate_AllFields(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewMenuRepository(mt.Client.Database("test"))
		mt.AddMockResponses(bson.D{
			{Key: "ok", Value: 1},
			{Key: "nModified", Value: 1},
			{Key: "n", Value: 1},
		})
		err := repo.Update("p-1", "Nuevo nombre", "Nueva desc", 15.0, true, "Nueva cat")
		assert.NoError(t, err)
	})
}

func TestMenuUpdate_Partial(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewMenuRepository(mt.Client.Database("test"))
		mt.AddMockResponses(bson.D{
			{Key: "ok", Value: 1},
			{Key: "nModified", Value: 1},
			{Key: "n", Value: 1},
		})
		err := repo.Update("p-1", "", "", 0, false, "")
		assert.NoError(t, err)
	})
}

func TestMenuUpdate_DBError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewMenuRepository(mt.Client.Database("test"))
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    50,
			Message: "error",
		}))
		err := repo.Update("p-99", "X", "", 0, true, "")
		assert.Error(t, err)
	})
}

func TestMenuDelete_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewMenuRepository(mt.Client.Database("test"))
		mt.AddMockResponses(bson.D{
			{Key: "ok", Value: 1},
			{Key: "n", Value: 1},
		})
		err := repo.Delete("p-1")
		assert.NoError(t, err)
	})
}

func TestMenuDelete_DBError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewMenuRepository(mt.Client.Database("test"))
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    50,
			Message: "error",
		}))
		err := repo.Delete("p-99")
		assert.Error(t, err)
	})
}

func TestMenuGetAdminID_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewMenuRepository(mt.Client.Database("test"))
		mt.AddMockResponses(
			mtest.CreateCursorResponse(0, "test.platos", mtest.FirstBatch,
				bson.D{
					{Key: "_id", Value: "p-1"},
					{Key: "id_restaurante", Value: "r-1"},
				},
			),
			mtest.CreateCursorResponse(0, "test.restaurantes", mtest.FirstBatch,
				bson.D{
					{Key: "_id", Value: "r-1"},
					{Key: "id_administrador", Value: "u-admin"},
				},
			),
		)
		adminID, err := repo.GetAdminID("p-1")
		assert.NoError(t, err)
		assert.Equal(t, "u-admin", adminID)
	})
}

func TestMenuGetAdminID_PlatoNotFound(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewMenuRepository(mt.Client.Database("test"))
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "test.platos", mtest.FirstBatch))
		adminID, err := repo.GetAdminID("p-99")
		assert.Error(t, err)
		assert.Equal(t, mongo.ErrNoDocuments, err)
		assert.Empty(t, adminID)
	})
}

func TestMenuGetAdminID_RestaurantNotFound(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewMenuRepository(mt.Client.Database("test"))
		mt.AddMockResponses(
			mtest.CreateCursorResponse(0, "test.platos", mtest.FirstBatch,
				bson.D{
					{Key: "_id", Value: "p-1"},
					{Key: "id_restaurante", Value: "r-99"},
				},
			),
			mtest.CreateCursorResponse(0, "test.restaurantes", mtest.FirstBatch),
		)
		adminID, err := repo.GetAdminID("p-1")
		assert.Error(t, err)
		assert.Equal(t, mongo.ErrNoDocuments, err)
		assert.Empty(t, adminID)
	})
}

// ============================================================
// ReservationRepository — 8 tests
// ============================================================

func TestReservationGetCapacity_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewReservationRepository(mt.Client.Database("test"))
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "test.restaurantes", mtest.FirstBatch,
			bson.D{
				{Key: "_id", Value: "r-1"},
				{Key: "capacidad_total", Value: int32(100)},
			},
		))
		cap, err := repo.GetRestaurantCapacity("r-1")
		assert.NoError(t, err)
		assert.Equal(t, 100, cap)
	})
}

func TestReservationGetCapacity_NotFound(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewReservationRepository(mt.Client.Database("test"))
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "test.restaurantes", mtest.FirstBatch))
		cap, err := repo.GetRestaurantCapacity("r-99")
		assert.Error(t, err)
		assert.Equal(t, mongo.ErrNoDocuments, err)
		assert.Equal(t, 0, cap)
	})
}

func TestReservationCreate_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewReservationRepository(mt.Client.Database("test"))
		now := time.Now()
		mt.AddMockResponses(mtest.CreateSuccessResponse(bson.E{Key: "n", Value: 1}))
		res, err := repo.Create("u-1", "r-1", now, 4)
		assert.NoError(t, err)
		assert.NotEmpty(t, res.ID)
		assert.Equal(t, "u-1", res.IdUsuario)
		assert.Equal(t, "r-1", res.IdRestaurante)
		assert.Equal(t, 4, res.CantidadPersonas)
		assert.Equal(t, 1, res.IdEstado)
		assert.Equal(t, "pendiente", res.NombreEstado)
	})
}

func TestReservationCreate_DBError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewReservationRepository(mt.Client.Database("test"))
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    11000,
			Message: "error",
		}))
		res, err := repo.Create("u-1", "r-1", time.Now(), 4)
		assert.Error(t, err)
		assert.Nil(t, res)
	})
}

func TestReservationGetOwnerID_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewReservationRepository(mt.Client.Database("test"))
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "test.reservas", mtest.FirstBatch,
			bson.D{
				{Key: "_id", Value: "res-1"},
				{Key: "id_usuario", Value: "u-1"},
			},
		))
		ownerID, err := repo.GetOwnerID("res-1")
		assert.NoError(t, err)
		assert.Equal(t, "u-1", ownerID)
	})
}

func TestReservationGetOwnerID_NotFound(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewReservationRepository(mt.Client.Database("test"))
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "test.reservas", mtest.FirstBatch))
		ownerID, err := repo.GetOwnerID("res-99")
		assert.Error(t, err)
		assert.Equal(t, mongo.ErrNoDocuments, err)
		assert.Empty(t, ownerID)
	})
}

func TestReservationCancel_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewReservationRepository(mt.Client.Database("test"))
		mt.AddMockResponses(bson.D{
			{Key: "ok", Value: 1},
			{Key: "nModified", Value: 1},
			{Key: "n", Value: 1},
		})
		err := repo.Cancel("res-1")
		assert.NoError(t, err)
	})
}

func TestReservationCancel_DBError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewReservationRepository(mt.Client.Database("test"))
		mt.AddMockResponses(mtest.CreateCommandErrorResponse(mtest.CommandError{
			Code:    50,
			Message: "error",
		}))
		err := repo.Cancel("res-99")
		assert.Error(t, err)
	})
}

// ============================================================
// OrderRepository — 4 tests
// ============================================================

func TestOrderCreate_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewOrderRepository(mt.Client.Database("test"))
		detalles := []models.OrderDetailRequest{
			{IdPlato: "p-1", Cantidad: 2},
		}
		mt.AddMockResponses(
			mtest.CreateCursorResponse(0, "test.platos", mtest.FirstBatch,
				bson.D{
					{Key: "_id", Value: "p-1"},
					{Key: "precio", Value: 10.5},
					{Key: "id_restaurante", Value: "r-1"},
					{Key: "nombre", Value: "Plato 1"},
					{Key: "descripcion", Value: "Desc"},
					{Key: "esta_disponible", Value: true},
					{Key: "categoria", Value: "X"},
				},
			),
			mtest.CreateSuccessResponse(bson.E{Key: "n", Value: 1}),
		)
		order, err := repo.Create("u-1", "r-1", false, detalles)
		assert.NoError(t, err)
		assert.NotEmpty(t, order.ID)
		assert.Equal(t, "u-1", order.IdUsuario)
		assert.Equal(t, false, order.EsRecogida)
		assert.Equal(t, 1, order.IdEstadoPedido)
		assert.Equal(t, 21.0, order.Total)
		assert.Len(t, order.Detalles, 1)
	})
}

func TestOrderCreate_DBError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewOrderRepository(mt.Client.Database("test"))
		detalles := []models.OrderDetailRequest{
			{IdPlato: "p-1", Cantidad: 1},
		}
		mt.AddMockResponses(
			mtest.CreateCursorResponse(0, "test.platos", mtest.FirstBatch,
				bson.D{
					{Key: "_id", Value: "p-1"},
					{Key: "precio", Value: 10.5},
					{Key: "id_restaurante", Value: "r-1"},
					{Key: "nombre", Value: "Plato 1"},
					{Key: "descripcion", Value: "Desc"},
					{Key: "esta_disponible", Value: true},
					{Key: "categoria", Value: "X"},
				},
			),
			mtest.CreateCommandErrorResponse(mtest.CommandError{
				Code:    50,
				Message: "error",
			}),
		)
		order, err := repo.Create("u-1", "r-1", true, detalles)
		assert.Error(t, err)
		assert.Nil(t, order)
	})
}

func TestOrderGetByID_Success(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewOrderRepository(mt.Client.Database("test"))
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "test.pedidos", mtest.FirstBatch,
			bson.D{
				{Key: "_id", Value: "ord-1"},
				{Key: "id_usuario", Value: "u-1"},
				{Key: "id_restaurante", Value: "r-1"},
				{Key: "es_recogida", Value: false},
				{Key: "id_estado_pedido", Value: int32(1)},
				{Key: "total", Value: 21.0},
				{Key: "detalles", Value: bson.A{}},
			},
		))
		order, err := repo.GetByID("ord-1")
		assert.NoError(t, err)
		assert.Equal(t, "ord-1", order.ID)
		assert.Equal(t, "u-1", order.IdUsuario)
		assert.Equal(t, 1, order.IdEstadoPedido)
		assert.Equal(t, 21.0, order.Total)
	})
}

func TestOrderGetByID_NotFound(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewOrderRepository(mt.Client.Database("test"))
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "test.pedidos", mtest.FirstBatch))
		order, err := repo.GetByID("ord-99")
		assert.Error(t, err)
		assert.Equal(t, mongo.ErrNoDocuments, err)
		assert.Nil(t, order)
	})
}

// ============================================================
// toInt coverage through UserRepository.GetByID — 6 tests
// ============================================================

func TestUserGetByID_RolInt32(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewUserRepository(mt.Client.Database("test"))
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "test.usuarios", mtest.FirstBatch,
			bson.D{
				{Key: "_id", Value: "u-1"},
				{Key: "nombre", Value: "A"},
				{Key: "email", Value: "a@b.com"},
				{Key: "id_rol", Value: int32(2)},
				{Key: "fecha_creacion", Value: primitive.NewDateTimeFromTime(time.Now())},
			},
		))
		user, err := repo.GetByID("u-1")
		assert.NoError(t, err)
		assert.Equal(t, 2, user.IdRol)
	})
}

func TestUserGetByID_RolInt(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewUserRepository(mt.Client.Database("test"))
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "test.usuarios", mtest.FirstBatch,
			bson.D{
				{Key: "_id", Value: "u-1"},
				{Key: "nombre", Value: "A"},
				{Key: "email", Value: "a@b.com"},
				{Key: "id_rol", Value: int(3)},
				{Key: "fecha_creacion", Value: primitive.NewDateTimeFromTime(time.Now())},
			},
		))
		user, err := repo.GetByID("u-1")
		assert.NoError(t, err)
		assert.Equal(t, 3, user.IdRol)
	})
}

func TestUserGetByID_RolInt64(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewUserRepository(mt.Client.Database("test"))
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "test.usuarios", mtest.FirstBatch,
			bson.D{
				{Key: "_id", Value: "u-1"},
				{Key: "nombre", Value: "A"},
				{Key: "email", Value: "a@b.com"},
				{Key: "id_rol", Value: int64(4)},
				{Key: "fecha_creacion", Value: primitive.NewDateTimeFromTime(time.Now())},
			},
		))
		user, err := repo.GetByID("u-1")
		assert.NoError(t, err)
		assert.Equal(t, 4, user.IdRol)
	})
}

func TestUserGetByID_RolFloat64(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewUserRepository(mt.Client.Database("test"))
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "test.usuarios", mtest.FirstBatch,
			bson.D{
				{Key: "_id", Value: "u-1"},
				{Key: "nombre", Value: "A"},
				{Key: "email", Value: "a@b.com"},
				{Key: "id_rol", Value: float64(5)},
				{Key: "fecha_creacion", Value: primitive.NewDateTimeFromTime(time.Now())},
			},
		))
		user, err := repo.GetByID("u-1")
		assert.NoError(t, err)
		assert.Equal(t, 5, user.IdRol)
	})
}

func TestUserGetByID_RolNil(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewUserRepository(mt.Client.Database("test"))
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "test.usuarios", mtest.FirstBatch,
			bson.D{
				{Key: "_id", Value: "u-1"},
				{Key: "nombre", Value: "A"},
				{Key: "email", Value: "a@b.com"},
				{Key: "fecha_creacion", Value: primitive.NewDateTimeFromTime(time.Now())},
			},
		))
		user, err := repo.GetByID("u-1")
		assert.NoError(t, err)
		assert.Equal(t, 0, user.IdRol)
	})
}

func TestUserGetByID_RolString(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("default", func(mt *mtest.T) {
		repo := repoMongo.NewUserRepository(mt.Client.Database("test"))
		mt.AddMockResponses(mtest.CreateCursorResponse(0, "test.usuarios", mtest.FirstBatch,
			bson.D{
				{Key: "_id", Value: "u-1"},
				{Key: "nombre", Value: "A"},
				{Key: "email", Value: "a@b.com"},
				{Key: "id_rol", Value: "admin"},
				{Key: "fecha_creacion", Value: primitive.NewDateTimeFromTime(time.Now())},
			},
		))
		user, err := repo.GetByID("u-1")
		assert.NoError(t, err)
		assert.Equal(t, 0, user.IdRol)
	})
}
