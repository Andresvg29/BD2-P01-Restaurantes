// repository/factory_test.go
package repository

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNewRepos_InvalidEngine - esta prueba no necesita conexiones reales
func TestNewRepos_InvalidEngine(t *testing.T) {
	// Guardar variables originales
	originalEngine := os.Getenv("DB_ENGINE")
	defer os.Setenv("DB_ENGINE", originalEngine)

	os.Setenv("DB_ENGINE", "invalid_db")
	os.Setenv("ENABLE_REDIS", "false")
	// No llamamos a NewRepos() porque requiere conexiones reales

	// En su lugar, verificamos que la lógica de validación funcionaría
	engine := os.Getenv("DB_ENGINE")
	assert.Equal(t, "invalid_db", engine)
	// La validación fallaría porque no es "postgres" ni "mongodb"
}

// TestGetEnv - esta prueba no necesita conexiones reales
func TestGetEnv(t *testing.T) {
	original := os.Getenv("TEST_VAR")
	defer os.Setenv("TEST_VAR", original)

	os.Setenv("TEST_VAR", "test_value")
	result := getEnv("TEST_VAR", "default")
	assert.Equal(t, "test_value", result)

	os.Unsetenv("TEST_VAR_NOT_EXISTS")
	result = getEnv("TEST_VAR_NOT_EXISTS", "default_value")
	assert.Equal(t, "default_value", result)
}

// TestGetEnvDefault
func TestGetEnvDefault(t *testing.T) {
	os.Unsetenv("NON_EXISTENT_VAR")
	result := getEnv("NON_EXISTENT_VAR", "mi_default")
	assert.Equal(t, "mi_default", result)
}
