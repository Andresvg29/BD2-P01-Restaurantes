package keycloak

import (
	"context"

	"github.com/Nerzal/gocloak/v13"
)

type ClientInterface interface {
	LoginUser(ctx context.Context, email, password string) (*gocloak.JWT, error)
	RegisterUser(ctx context.Context, nombre, apellido, email, password string) (string, error)
}
