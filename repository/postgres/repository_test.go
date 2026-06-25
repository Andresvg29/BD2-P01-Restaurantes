package postgres_test

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"restaurante/models"
	"restaurante/repository/postgres"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

// helper: crea un *sql.DB con sqlmock listo para usar
func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error creando sqlmock: %v", err)
	}
	return db, mock
}

// ── UserRepository ────────────────────────────────────────────────────────────

func TestUserGetByID_OK(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	now := time.Now()
	rows := sqlmock.NewRows([]string{"id", "nombre", "email", "id_rol", "fecha_creacion"}).
		AddRow("user-1", "Ana", "ana@mail.com", 2, now)

	mock.ExpectQuery(`SELECT id, nombre, email, id_rol, fecha_creacion FROM usuarios WHERE id`).
		WithArgs("user-1").
		WillReturnRows(rows)

	repo := postgres.NewUserRepository(db)
	user, err := repo.GetByID("user-1")

	assert.NoError(t, err)
	assert.Equal(t, "user-1", user.ID)
	assert.Equal(t, "Ana", user.Nombre)
	assert.Equal(t, "ana@mail.com", user.Email)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUserGetByID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT id, nombre, email, id_rol, fecha_creacion FROM usuarios WHERE id`).
		WithArgs("user-999").
		WillReturnError(sql.ErrNoRows)

	repo := postgres.NewUserRepository(db)
	user, err := repo.GetByID("user-999")

	assert.Error(t, err)
	assert.Nil(t, user)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUserGetByEmail_OK(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id"}).AddRow("user-1")
	mock.ExpectQuery(`SELECT id FROM usuarios WHERE email`).
		WithArgs("ana@mail.com").
		WillReturnRows(rows)

	repo := postgres.NewUserRepository(db)
	user, err := repo.GetByEmail("ana@mail.com")

	assert.NoError(t, err)
	assert.Equal(t, "user-1", user.ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUserGetByEmail_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT id FROM usuarios WHERE email`).
		WithArgs("noexiste@mail.com").
		WillReturnError(sql.ErrNoRows)

	repo := postgres.NewUserRepository(db)
	user, err := repo.GetByEmail("noexiste@mail.com")

	assert.Error(t, err)
	assert.Nil(t, user)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUserCreate_OK(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id"}).AddRow("new-user-1")
	mock.ExpectQuery(`INSERT INTO usuarios`).
		WithArgs("Ana", "ana@mail.com", "keycloak-id-123").
		WillReturnRows(rows)

	repo := postgres.NewUserRepository(db)
	id, err := repo.Create("Ana", "ana@mail.com", "keycloak-id-123")

	assert.NoError(t, err)
	assert.Equal(t, "new-user-1", id)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUserCreate_DBError(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(`INSERT INTO usuarios`).
		WithArgs("Ana", "ana@mail.com", "keycloak-id-123").
		WillReturnError(errors.New("duplicate key"))

	repo := postgres.NewUserRepository(db)
	id, err := repo.Create("Ana", "ana@mail.com", "keycloak-id-123")

	assert.Error(t, err)
	assert.Empty(t, id)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUserUpdate_OK(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(`UPDATE usuarios SET`).
		WithArgs("Nuevo Nombre", "nuevo@mail.com", "user-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	repo := postgres.NewUserRepository(db)
	err := repo.Update("user-1", "Nuevo Nombre", "nuevo@mail.com")

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUserUpdate_DBError(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(`UPDATE usuarios SET`).
		WithArgs("Nombre", "", "user-1").
		WillReturnError(errors.New("db error"))

	repo := postgres.NewUserRepository(db)
	err := repo.Update("user-1", "Nombre", "")

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUserDelete_OK(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(`DELETE FROM usuarios WHERE id`).
		WithArgs("user-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	repo := postgres.NewUserRepository(db)
	err := repo.Delete("user-1")

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUserDelete_DBError(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(`DELETE FROM usuarios WHERE id`).
		WithArgs("user-1").
		WillReturnError(errors.New("db error"))

	repo := postgres.NewUserRepository(db)
	err := repo.Delete("user-1")

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUserUpsertFromToken_Cliente(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(`INSERT INTO usuarios`).
		WithArgs("kc-id", "Ana", "ana@mail.com", "kc-id", 2).
		WillReturnResult(sqlmock.NewResult(1, 1))

	repo := postgres.NewUserRepository(db)
	err := repo.UpsertFromToken("kc-id", "Ana", "ana@mail.com", false)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUserUpsertFromToken_Admin(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(`INSERT INTO usuarios`).
		WithArgs("kc-id", "Admin", "admin@mail.com", "kc-id", 1).
		WillReturnResult(sqlmock.NewResult(1, 1))

	repo := postgres.NewUserRepository(db)
	err := repo.UpsertFromToken("kc-id", "Admin", "admin@mail.com", true)

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── RestaurantRepository ──────────────────────────────────────────────────────

func TestRestaurantList_OK(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "nombre", "direccion", "telefono", "id_administrador", "capacidad_total"}).
		AddRow("r-1", "La Trattoria", "San José", "88881234", "user-1", 40).
		AddRow("r-2", "El Fogón", "Cartago", "99990000", "user-2", 30)

	mock.ExpectQuery(`SELECT id, nombre, direccion, telefono, id_administrador, capacidad_total FROM restaurantes`).
		WillReturnRows(rows)

	repo := postgres.NewRestaurantRepository(db, nil)
	list, err := repo.List()

	assert.NoError(t, err)
	assert.Len(t, list, 2)
	assert.Equal(t, "La Trattoria", list[0].Nombre)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRestaurantList_Empty(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "nombre", "direccion", "telefono", "id_administrador", "capacidad_total"})
	mock.ExpectQuery(`SELECT id, nombre, direccion, telefono, id_administrador, capacidad_total FROM restaurantes`).
		WillReturnRows(rows)

	repo := postgres.NewRestaurantRepository(db, nil)
	list, err := repo.List()

	assert.NoError(t, err)
	assert.Empty(t, list)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRestaurantList_DBError(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT id, nombre, direccion, telefono, id_administrador, capacidad_total FROM restaurantes`).
		WillReturnError(errors.New("db error"))

	repo := postgres.NewRestaurantRepository(db, nil)
	list, err := repo.List()

	assert.Error(t, err)
	assert.Nil(t, list)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRestaurantGetByID_OK(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "nombre", "direccion", "telefono", "id_administrador", "capacidad_total"}).
		AddRow("r-1", "La Trattoria", "San José", "88881234", "user-1", 40)

	mock.ExpectQuery(`SELECT id, nombre, direccion, telefono, id_administrador, capacidad_total FROM restaurantes WHERE id`).
		WithArgs("r-1").
		WillReturnRows(rows)

	repo := postgres.NewRestaurantRepository(db, nil)
	rest, err := repo.GetByID("r-1")

	assert.NoError(t, err)
	assert.Equal(t, "r-1", rest.ID)
	assert.Equal(t, 40, rest.CapacidadTotal)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRestaurantGetByID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT id, nombre, direccion, telefono, id_administrador, capacidad_total FROM restaurantes WHERE id`).
		WithArgs("r-999").
		WillReturnError(sql.ErrNoRows)

	repo := postgres.NewRestaurantRepository(db, nil)
	rest, err := repo.GetByID("r-999")

	assert.Error(t, err)
	assert.Nil(t, rest)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRestaurantCreate_OK(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id"}).AddRow("new-rest-1")
	mock.ExpectQuery(`INSERT INTO restaurantes`).
		WithArgs("La Trattoria", "San José", "88881234", "user-1", 40).
		WillReturnRows(rows)

	repo := postgres.NewRestaurantRepository(db, nil)
	id, err := repo.Create("La Trattoria", "San José", "88881234", "user-1", 40)

	assert.NoError(t, err)
	assert.Equal(t, "new-rest-1", id)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── MenuRepository ────────────────────────────────────────────────────────────

func TestMenuList_OK(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "id_restaurante", "nombre", "descripcion", "precio", "esta_disponible", "categoria"}).
		AddRow("p-1", "r-1", "Pizza", "Rica pizza", 5000.0, true, "comida").
		AddRow("p-2", "r-1", "Pasta", "Rica pasta", 4500.0, true, "comida")

	mock.ExpectQuery(`SELECT id, id_restaurante, nombre, descripcion, precio, esta_disponible, categoria FROM platos`).
		WillReturnRows(rows)

	repo := postgres.NewMenuRepository(db)
	items, err := repo.List()

	assert.NoError(t, err)
	assert.Len(t, items, 2)
	assert.Equal(t, "Pizza", items[0].Nombre)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMenuList_DBError(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT id, id_restaurante, nombre, descripcion, precio, esta_disponible, categoria FROM platos`).
		WillReturnError(errors.New("db error"))

	repo := postgres.NewMenuRepository(db)
	items, err := repo.List()

	assert.Error(t, err)
	assert.Nil(t, items)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMenuGetByRestaurant_OK(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "id_restaurante", "nombre", "descripcion", "precio", "esta_disponible", "categoria"}).
		AddRow("p-1", "r-1", "Pizza", "Rica pizza", 5000.0, true, "comida")

	mock.ExpectQuery(`SELECT id, id_restaurante, nombre, descripcion, precio, esta_disponible, categoria`).
		WithArgs("r-1").
		WillReturnRows(rows)

	repo := postgres.NewMenuRepository(db)
	items, err := repo.GetByRestaurant("r-1")

	assert.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, "Pizza", items[0].Nombre)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMenuGetByRestaurant_Empty(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "id_restaurante", "nombre", "descripcion", "precio", "esta_disponible", "categoria"})
	mock.ExpectQuery(`SELECT id, id_restaurante, nombre, descripcion, precio, esta_disponible, categoria`).
		WithArgs("r-1").
		WillReturnRows(rows)

	repo := postgres.NewMenuRepository(db)
	items, err := repo.GetByRestaurant("r-1")

	assert.NoError(t, err)
	assert.Empty(t, items)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMenuCreate_OK(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id"}).AddRow("new-plato-1")
	mock.ExpectQuery(`INSERT INTO platos`).
		WithArgs("r-1", "Pizza", "Rica pizza", 5000.0, "comida").
		WillReturnRows(rows)

	repo := postgres.NewMenuRepository(db)
	id, err := repo.Create("r-1", "Pizza", "Rica pizza", 5000.0, "comida")

	assert.NoError(t, err)
	assert.Equal(t, "new-plato-1", id)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMenuCreate_DefaultDescription(t *testing.T) {
	// when descripcion is empty it should use "Producto sin descripcion"
	db, mock := newMockDB(t)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id"}).AddRow("new-plato-2")
	mock.ExpectQuery(`INSERT INTO platos`).
		WithArgs("r-1", "Pizza", "Producto sin descripcion", 5000.0, "comida").
		WillReturnRows(rows)

	repo := postgres.NewMenuRepository(db)
	id, err := repo.Create("r-1", "Pizza", "", 5000.0, "comida")

	assert.NoError(t, err)
	assert.Equal(t, "new-plato-2", id)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMenuCreate_DBError(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(`INSERT INTO platos`).
		WithArgs("r-1", "Pizza", "Rica pizza", 5000.0, "comida").
		WillReturnError(errors.New("db error"))

	repo := postgres.NewMenuRepository(db)
	id, err := repo.Create("r-1", "Pizza", "Rica pizza", 5000.0, "comida")

	assert.Error(t, err)
	assert.Empty(t, id)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMenuUpdate_OK(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(`UPDATE platos SET`).
		WithArgs("Pizza XL", "Descripcion nueva", 6000.0, true, "comida", "p-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	repo := postgres.NewMenuRepository(db)
	err := repo.Update("p-1", "Pizza XL", "Descripcion nueva", 6000.0, true, "comida")

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMenuUpdate_DBError(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(`UPDATE platos SET`).
		WithArgs("Pizza XL", "", 6000.0, true, "", "p-1").
		WillReturnError(errors.New("db error"))

	repo := postgres.NewMenuRepository(db)
	err := repo.Update("p-1", "Pizza XL", "", 6000.0, true, "")

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMenuDelete_OK(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(`DELETE FROM platos WHERE id`).
		WithArgs("p-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	repo := postgres.NewMenuRepository(db)
	err := repo.Delete("p-1")

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMenuDelete_DBError(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectExec(`DELETE FROM platos WHERE id`).
		WithArgs("p-1").
		WillReturnError(errors.New("db error"))

	repo := postgres.NewMenuRepository(db)
	err := repo.Delete("p-1")

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMenuGetAdminID_OK(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id_administrador"}).AddRow("user-1")
	mock.ExpectQuery(`SELECT r.id_administrador FROM restaurantes r`).
		WithArgs("p-1").
		WillReturnRows(rows)

	repo := postgres.NewMenuRepository(db)
	adminID, err := repo.GetAdminID("p-1")

	assert.NoError(t, err)
	assert.Equal(t, "user-1", adminID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestMenuGetAdminID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT r.id_administrador FROM restaurantes r`).
		WithArgs("p-999").
		WillReturnError(sql.ErrNoRows)

	repo := postgres.NewMenuRepository(db)
	adminID, err := repo.GetAdminID("p-999")

	assert.Error(t, err)
	assert.Empty(t, adminID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── ReservationRepository ─────────────────────────────────────────────────────

func TestReservationGetCapacity_OK(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"capacidad_total"}).AddRow(50)
	mock.ExpectQuery(`SELECT capacidad_total FROM restaurantes WHERE id`).
		WithArgs("r-1").
		WillReturnRows(rows)

	repo := postgres.NewReservationRepository(db)
	cap, err := repo.GetRestaurantCapacity("r-1")

	assert.NoError(t, err)
	assert.Equal(t, 50, cap)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestReservationGetCapacity_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT capacidad_total FROM restaurantes WHERE id`).
		WithArgs("r-999").
		WillReturnError(sql.ErrNoRows)

	repo := postgres.NewReservationRepository(db)
	cap, err := repo.GetRestaurantCapacity("r-999")

	assert.Error(t, err)
	assert.Equal(t, 0, cap)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestReservationCreate_OK(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	fecha := time.Now().Add(24 * time.Hour)

	// query para estado 'pendiente'
	stateRows := sqlmock.NewRows([]string{"id"}).AddRow(1)
	mock.ExpectQuery(`SELECT id FROM estados_reserva WHERE nombre_estado`).
		WillReturnRows(stateRows)

	// insert reservation
	resRows := sqlmock.NewRows([]string{"id"}).AddRow("res-1")
	mock.ExpectQuery(`INSERT INTO reservas`).
		WithArgs("user-1", "r-1", fecha, 4, 1).
		WillReturnRows(resRows)

	repo := postgres.NewReservationRepository(db)
	res, err := repo.Create("user-1", "r-1", fecha, 4)

	assert.NoError(t, err)
	assert.Equal(t, "res-1", res.ID)
	assert.Equal(t, "pendiente", res.NombreEstado)
	assert.Equal(t, 4, res.CantidadPersonas)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestReservationCreate_DBError(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	fecha := time.Now().Add(24 * time.Hour)

	stateRows := sqlmock.NewRows([]string{"id"}).AddRow(1)
	mock.ExpectQuery(`SELECT id FROM estados_reserva WHERE nombre_estado`).
		WillReturnRows(stateRows)

	mock.ExpectQuery(`INSERT INTO reservas`).
		WithArgs("user-1", "r-1", fecha, 4, 1).
		WillReturnError(errors.New("db error"))

	repo := postgres.NewReservationRepository(db)
	res, err := repo.Create("user-1", "r-1", fecha, 4)

	assert.Error(t, err)
	assert.Nil(t, res)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestReservationGetOwnerID_OK(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id_usuario"}).AddRow("user-1")
	mock.ExpectQuery(`SELECT id_usuario FROM reservas WHERE id`).
		WithArgs("res-1").
		WillReturnRows(rows)

	repo := postgres.NewReservationRepository(db)
	ownerID, err := repo.GetOwnerID("res-1")

	assert.NoError(t, err)
	assert.Equal(t, "user-1", ownerID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestReservationCancel_OK(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	stateRows := sqlmock.NewRows([]string{"id"}).AddRow(3)
	mock.ExpectQuery(`SELECT id FROM estados_reserva WHERE nombre_estado`).
		WillReturnRows(stateRows)

	mock.ExpectExec(`UPDATE reservas SET id_estado`).
		WithArgs(3, "res-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	repo := postgres.NewReservationRepository(db)
	err := repo.Cancel("res-1")

	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestReservationCancel_DBError(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	stateRows := sqlmock.NewRows([]string{"id"}).AddRow(3)
	mock.ExpectQuery(`SELECT id FROM estados_reserva WHERE nombre_estado`).
		WillReturnRows(stateRows)

	mock.ExpectExec(`UPDATE reservas SET id_estado`).
		WithArgs(3, "res-1").
		WillReturnError(errors.New("db error"))

	repo := postgres.NewReservationRepository(db)
	err := repo.Cancel("res-1")

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ── OrderRepository ───────────────────────────────────────────────────────────

func TestOrderGetByID_OK(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	orderRows := sqlmock.NewRows([]string{"id", "id_usuario", "id_restaurante", "es_recogida", "id_estado_pedido", "total"}).
		AddRow("order-1", "user-1", "r-1", false, 1, 10000.0)
	mock.ExpectQuery(`SELECT id, id_usuario, id_restaurante, es_recogida, id_estado_pedido, total FROM pedidos WHERE id`).
		WithArgs("order-1").
		WillReturnRows(orderRows)

	detailRows := sqlmock.NewRows([]string{"id", "id_plato", "cantidad", "precio"}).
		AddRow("det-1", "p-1", 2, 5000.0)
	mock.ExpectQuery(`SELECT id, id_plato, cantidad, precio FROM detalles_pedido WHERE id_pedido`).
		WithArgs("order-1").
		WillReturnRows(detailRows)

	repo := postgres.NewOrderRepository(db)
	order, err := repo.GetByID("order-1")

	assert.NoError(t, err)
	assert.Equal(t, "order-1", order.ID)
	assert.Equal(t, 10000.0, order.Total)
	assert.Len(t, order.Detalles, 1)
	assert.Equal(t, "p-1", order.Detalles[0].IdPlato)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOrderGetByID_NotFound(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT id, id_usuario, id_restaurante, es_recogida, id_estado_pedido, total FROM pedidos WHERE id`).
		WithArgs("order-999").
		WillReturnError(sql.ErrNoRows)

	repo := postgres.NewOrderRepository(db)
	order, err := repo.GetByID("order-999")

	assert.Error(t, err)
	assert.Nil(t, order)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOrderCreate_OK(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	detalles := []models.OrderDetailRequest{
		{IdPlato: "p-1", Cantidad: 2},
	}

	// estado pendiente
	stateRows := sqlmock.NewRows([]string{"id"}).AddRow(1)
	mock.ExpectQuery(`SELECT id FROM estados_pedido WHERE nombre_estado`).
		WillReturnRows(stateRows)

	mock.ExpectBegin()

	// insert order
	orderRows := sqlmock.NewRows([]string{"id"}).AddRow("order-1")
	mock.ExpectQuery(`INSERT INTO pedidos`).
		WithArgs("user-1", "r-1", false, 1).
		WillReturnRows(orderRows)

	// dish price
	precioRows := sqlmock.NewRows([]string{"precio"}).AddRow(5000.0)
	mock.ExpectQuery(`SELECT precio FROM platos WHERE id`).
		WithArgs("p-1", "r-1").
		WillReturnRows(precioRows)

	// insert detalle
	detailRows := sqlmock.NewRows([]string{"id"}).AddRow("det-1")
	mock.ExpectQuery(`INSERT INTO detalles_pedido`).
		WithArgs("order-1", "p-1", 2, 5000.0).
		WillReturnRows(detailRows)

	// update total
	mock.ExpectExec(`UPDATE pedidos SET total`).
		WithArgs(10000.0, "order-1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectCommit()

	repo := postgres.NewOrderRepository(db)
	order, err := repo.Create("user-1", "r-1", false, detalles)

	assert.NoError(t, err)
	assert.Equal(t, "order-1", order.ID)
	assert.Equal(t, 10000.0, order.Total)
	assert.Len(t, order.Detalles, 1)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOrderCreate_TransactionError(t *testing.T) {
	db, mock := newMockDB(t)
	defer db.Close()

	detalles := []models.OrderDetailRequest{
		{IdPlato: "p-1", Cantidad: 2},
	}

	stateRows := sqlmock.NewRows([]string{"id"}).AddRow(1)
	mock.ExpectQuery(`SELECT id FROM estados_pedido WHERE nombre_estado`).
		WillReturnRows(stateRows)

	mock.ExpectBegin().WillReturnError(errors.New("tx error"))

	repo := postgres.NewOrderRepository(db)
	order, err := repo.Create("user-1", "r-1", false, detalles)

	assert.Error(t, err)
	assert.Nil(t, order)
	assert.NoError(t, mock.ExpectationsWereMet())
}
