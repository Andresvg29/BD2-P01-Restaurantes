package repository

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

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

func TestGetEnvDefault(t *testing.T) {
	os.Unsetenv("NON_EXISTENT_VAR")
	result := getEnv("NON_EXISTENT_VAR", "mi_default")
	assert.Equal(t, "mi_default", result)
}

func TestGetEnvEmptyString(t *testing.T) {
	os.Setenv("EMPTY_VAR_TEST", "")
	result := getEnv("EMPTY_VAR_TEST", "fallback")
	assert.Equal(t, "fallback", result, "empty string should return fallback")
}
