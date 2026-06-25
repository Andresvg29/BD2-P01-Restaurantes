package postgres

import (
	"database/sql"

	"restaurante/models"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) GetByID(id string) (*models.User, error) {
	u := &models.User{}
	err := r.db.QueryRow(
		`SELECT id, nombre, email, id_rol, fecha_creacion FROM usuarios WHERE id = $1`, id,
	).Scan(&u.ID, &u.Nombre, &u.Email, &u.IdRol, &u.FechaCreacion)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (r *UserRepository) GetByEmail(email string) (*models.User, error) {
	u := &models.User{}
	err := r.db.QueryRow(
		`SELECT id FROM usuarios WHERE email = $1`, email,
	).Scan(&u.ID)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (r *UserRepository) Create(nombre, email, keycloakID string) (string, error) {
	var id string
	err := r.db.QueryRow(
		`INSERT INTO usuarios (nombre, email, keycloak_id, id_rol)
		 VALUES ($1, $2, $3, (SELECT id FROM roles_usuario WHERE nombre_rol = 'cliente'))
		 RETURNING id`,
		nombre, email, keycloakID,
	).Scan(&id)
	return id, err
}

func (r *UserRepository) Update(id, nombre, email string) error {
	_, err := r.db.Exec(
		`UPDATE usuarios SET
			nombre = COALESCE(NULLIF($1, ''), nombre),
			email  = COALESCE(NULLIF($2, ''), email)
		 WHERE id = $3`,
		nombre, email, id,
	)
	return err
}

func (r *UserRepository) Delete(id string) error {
	_, err := r.db.Exec(`DELETE FROM usuarios WHERE id = $1`, id)
	return err
}

func (r *UserRepository) UpsertFromToken(id, nombre, email string, isAdmin bool) error {
	rolID := 2
	if isAdmin {
		rolID = 1
	}

	query := `
    INSERT INTO usuarios (id, nombre, email, keycloak_id, id_rol)
    VALUES ($1, $2, $3, $4, $5)
    ON CONFLICT (id) DO UPDATE 
    SET nombre = $2, email = $3, id_rol = $5
`

	_, err := r.db.Exec(query, id, nombre, email, id, rolID)
	return err
}
