package keycloak

import (
	"context"

	"github.com/Nerzal/gocloak/v13"
)

// ClientInterface define las operaciones de Keycloak que necesita la app.
// Se utiliza para poder mockear en tests unitarios.
type ClientInterface interface {
	LoginUser(ctx context.Context, email, password string) (*gocloak.JWT, error)
	RegisterUser(ctx context.Context, nombre, apellido, email, password string) (string, error)
}
