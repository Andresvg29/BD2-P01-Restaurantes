package keycloak

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/Nerzal/gocloak/v13"
	"github.com/golang-jwt/jwt/v5"
)

// envuelve gocloak y expone las operaciones que necesita la app
type Client struct {
	kc           gocloak.GoCloak
	realm        string
	clientID     string
	clientSecret string
}

// crea un cliente de Keycloak listo para usarse
func New() *Client {
	return &Client{
		kc:           *gocloak.NewClient(os.Getenv("KEYCLOAK_URL")),
		realm:        os.Getenv("KEYCLOAK_REALM"),
		clientID:     os.Getenv("KEYCLOAK_CLIENT_ID"),
		clientSecret: os.Getenv("KEYCLOAK_CLIENT_SECRET"),
	}
}

// verifica que el realm, cliente y roles existan en Keycloak sino los crea
func (c *Client) Setup(ctx context.Context) error {
	token, err := c.kc.LoginAdmin(ctx,
		os.Getenv("KEYCLOAK_ADMIN"),
		os.Getenv("KEYCLOAK_ADMIN_PASSWORD"),
		"master",
	)
	if err != nil {
		return fmt.Errorf("login admin keycloak: %w", err)
	}

	if err := c.ensureRealm(ctx, token.AccessToken); err != nil {
		return err
	}
	if err := c.ensureClient(ctx, token.AccessToken); err != nil {
		return err
	}
	if err := c.ensureRoles(ctx, token.AccessToken); err != nil {
		return err
	}

	log.Println("Keycloak configurado correctamente")
	return nil
}

func (c *Client) ensureRealm(ctx context.Context, adminToken string) error {
	realms, err := c.kc.GetRealms(ctx, adminToken)
	if err != nil {
		return fmt.Errorf("listar realms: %w", err)
	}
	for _, r := range realms {
		if r.Realm != nil && *r.Realm == c.realm {
			log.Printf("  realm '%s' ya existe", c.realm)
			return nil
		}
	}
	enabled := true
	_, err = c.kc.CreateRealm(ctx, adminToken, gocloak.RealmRepresentation{
		Realm:   &c.realm,
		Enabled: &enabled,
	})
	if err != nil {
		return fmt.Errorf("crear realm: %w", err)
	}
	log.Printf("  realm '%s' creado", c.realm)
	return nil
}

func (c *Client) ensureClient(ctx context.Context, adminToken string) error {
	clients, err := c.kc.GetClients(ctx, adminToken, c.realm,
		gocloak.GetClientsParams{ClientID: &c.clientID},
	)
	if err != nil {
		return fmt.Errorf("listar clientes: %w", err)
	}
	if len(clients) > 0 {
		log.Printf("  cliente '%s' ya existe", c.clientID)
		return nil
	}
	t := true
	f := false
	secret := c.clientSecret
	_, err = c.kc.CreateClient(ctx, adminToken, c.realm, gocloak.Client{
		ClientID:                  &c.clientID,
		Secret:                    &secret,
		Enabled:                   &t,
		DirectAccessGrantsEnabled: &t,
		PublicClient:              &f,
		StandardFlowEnabled:       &f,
	})
	if err != nil {
		return fmt.Errorf("crear cliente: %w", err)
	}
	log.Printf("  cliente '%s' creado", c.clientID)
	return nil
}

func (c *Client) ensureRoles(ctx context.Context, adminToken string) error {
	for _, roleName := range []string{"admin", "cliente"} {
		_, err := c.kc.GetRealmRole(ctx, adminToken, c.realm, roleName)
		if err == nil {
			log.Printf("  rol '%s' ya existe", roleName)
			continue
		}
		name := roleName
		if _, err := c.kc.CreateRealmRole(ctx, adminToken, c.realm, gocloak.Role{Name: &name}); err != nil {
			return fmt.Errorf("crear rol '%s': %w", roleName, err)
		}
		log.Printf("  rol '%s' creado", roleName)
	}
	return nil
}

// LoginUser obtiene un JWT para el usuario con email/password
func (c *Client) LoginUser(ctx context.Context, email, password string) (*gocloak.JWT, error) {
	token, err := c.kc.Login(ctx, c.clientID, c.clientSecret, c.realm, email, password)
	if err != nil {
		return nil, fmt.Errorf("credenciales inválidas")
	}
	return token, nil
}

// RegisterUser crea el usuario en Keycloak y le asigna el rol 'cliente'
func (c *Client) RegisterUser(ctx context.Context, nombre, apellido, email, password string) (string, error) {
	adminToken, err := c.kc.LoginAdmin(ctx,
		os.Getenv("KEYCLOAK_ADMIN"),
		os.Getenv("KEYCLOAK_ADMIN_PASSWORD"),
		"master",
	)
	if err != nil {
		return "", fmt.Errorf("error autenticando admin")
	}

	t := true
	empty := []string{}
	userID, err := c.kc.CreateUser(ctx, adminToken.AccessToken, c.realm, gocloak.User{
		Username:        &email,
		Email:           &email,
		FirstName:       &nombre,
		LastName:        &apellido,
		Enabled:         &t,
		EmailVerified:   &t,
		RequiredActions: &empty,
		Credentials: &[]gocloak.CredentialRepresentation{
			{
				Type:      gocloak.StringP("password"),
				Value:     &password,
				Temporary: gocloak.BoolP(false),
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("email ya registrado o datos invalidos")
	}

	// Asignar rol 'cliente'
	if role, err := c.kc.GetRealmRole(ctx, adminToken.AccessToken, c.realm, "cliente"); err == nil {
		c.kc.AddRealmRoleToUser(ctx, adminToken.AccessToken, c.realm, userID, []gocloak.Role{*role})
	}

	return userID, nil
}

// verifica la firma del JWT con las claves publicas de Keycloak
func (c *Client) DecodeToken(ctx context.Context, tokenString string) (userID, userName, userEmail string, err error) {
	jwtToken, mapClaims, err := c.kc.DecodeAccessToken(ctx, tokenString, c.realm)
	if err != nil || !jwtToken.Valid {
		return "", "", "", fmt.Errorf("token inválido: %w", err)
	}

	claims := *mapClaims

	// "sub" es el UUID del usuario en Keycloak
	if sub, ok := claims["sub"].(string); ok {
		userID = sub
	}
	// "preferred_username" es el nombre de usuario / email usado al registrar
	if un, ok := claims["preferred_username"].(string); ok {
		userName = un
	}
	// "email" puede no estar presente si el scope no lo incluye
	if em, ok := claims["email"].(string); ok {
		userEmail = em
	}

	if userID == "" {
		return "", "", "", fmt.Errorf("claim 'sub' no encontrado en el token")
	}

	return userID, userName, userEmail, nil
}

// lee los roles del realm desde los claims del JWT
func ExtractRoles(claims jwt.MapClaims) []string {
	ra, ok := claims["realm_access"].(map[string]interface{})
	if !ok {
		return nil
	}
	rawRoles, ok := ra["roles"].([]interface{})
	if !ok {
		return nil
	}
	roles := make([]string, 0, len(rawRoles))
	for _, r := range rawRoles {
		if s, ok := r.(string); ok {
			roles = append(roles, s)
		}
	}
	return roles
}
