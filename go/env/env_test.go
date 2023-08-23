package env_test

import (
	"os"
	"runtime"
	"testing"

	"github.com/kodekoding/phastos/v2/go/env"
	"github.com/stretchr/testify/require"
)

func TestSetFromEnvFile(t *testing.T) {
	err := env.SetFromEnvFile("testfile/.env")
	if err != nil {
		t.Error(err)
	}
	val1 := os.Getenv("KEY1")
	if val1 != "value1" {
		t.Errorf("Invalid KEY1 value. Value: %s", val1)
	}
	val2 := os.Getenv("KEY2")
	if val2 != "value2" {
		t.Errorf("Invalid KEY2 value. Value: %s", val2)
	}
	val3 := os.Getenv("KEY3")
	if val3 != "value3" {
		t.Errorf("Invalid KEY3 value. Value: %s", val3)
	}
}

func TestGetSet(t *testing.T) {
	cases := []struct {
		key   string
		value string
	}{
		{
			key:   "key1",
			value: "value1",
		},
		{
			key:   "key2",
			value: "value2",
		},
		{
			key:   "key3",
			value: "value3",
		},
	}

	for _, c := range cases {
		err := os.Setenv(c.key, c.value)
		if err != nil {
			t.Error(err)
		}
		val := os.Getenv(c.key)
		if val != c.value {
			t.Errorf("Expecting %s but got %s", c.value, val)
		}
	}
}

func TestServiceEnv(t *testing.T) {
	// test no TKPENV -> development
	require.Equal(t, env.DevelopmentEnv, env.ServiceEnv())
	require.True(t, env.IsDevelopment())
	require.False(t, env.IsStaging())

	// set to staging
	os.Setenv("TKPENV", "staging")
	require.Equal(t, env.StagingEnv, env.ServiceEnv())
	require.False(t, env.IsDevelopment())
	require.True(t, env.IsStaging())

	// set to production
	os.Setenv("TKPENV", "production")
	require.Equal(t, env.ProductionEnv, env.ServiceEnv())
	require.False(t, env.IsDevelopment())
	require.False(t, env.IsStaging())
	require.True(t, env.IsProduction())
}

func TestGoVersion(t *testing.T) {
	require.Equal(t, runtime.Version(), env.GoVersion())
}
