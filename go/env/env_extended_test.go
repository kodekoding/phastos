package env

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsLocal(t *testing.T) {
	os.Setenv("APPS_ENV", "local")
	assert.True(t, IsLocal())
	assert.False(t, IsDevelopment())
}

func TestSetFromEnvFileNonExistent(t *testing.T) {
	err := SetFromEnvFile("/nonexistent/.env")
	assert.True(t, os.IsNotExist(err))
}

func TestSetFromEnvFileEmptyPath(t *testing.T) {
	// Empty path should return stat error
	err := SetFromEnvFile("")
	assert.Error(t, err)
}

func TestServiceNameEnvType(t *testing.T) {
	var s ServiceNameEnv = "test"
	assert.Equal(t, "test", s)
}

func TestEnvironmentConstants(t *testing.T) {
	assert.Equal(t, "development", DevelopmentEnv)
	assert.Equal(t, "staging", StagingEnv)
	assert.Equal(t, "production", ProductionEnv)
	assert.Equal(t, "local", LocalEnv)
}

func TestServiceEnvWhenEmpty(t *testing.T) {
	os.Unsetenv("APPS_ENV")
	result := ServiceEnv()
	assert.Equal(t, DevelopmentEnv, result)
}

func TestSetFromEnvFile_OpenError(t *testing.T) {
	orig := osOpenFile
	defer func() { osOpenFile = orig }()

	osOpenFile = func(name string) (*os.File, error) {
		return nil, os.ErrPermission
	}

	err := SetFromEnvFile("testfile/.env")
	require.Error(t, err)
	require.True(t, errors.Is(err, os.ErrPermission))
}
