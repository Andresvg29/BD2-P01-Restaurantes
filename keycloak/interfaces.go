package keycloak

import (
	"context"

	"github.com/Nerzal/gocloak/v13"
)

// ClientInterface defines the Keycloak operations needed by the app.
// It is used to enable mocking in unit tests.
type ClientInterface interface {
	LoginUser(ctx context.Context, email, password string) (*gocloak.JWT, error)
	RegisterUser(ctx context.Context, nombre, apellido, email, password string) (string, error)
}
