package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewConfigHasSequentialConcurrencyDefault(t *testing.T) {
	cfg := NewConfig()
	require.Equal(t, 1, cfg.Workflow.SequentialMaxConcurrency)
}

func TestValidateSequentialConcurrency(t *testing.T) {
	t.Run("invalid_zero", func(t *testing.T) {
		cfg := NewConfig()
		cfg.Workflow.SequentialMaxConcurrency = 0
		errs := cfg.Validate()
		require.NotEmpty(t, errs)
	})

	t.Run("valid_positive", func(t *testing.T) {
		cfg := NewConfig()
		cfg.Workflow.SequentialMaxConcurrency = 4
		errs := cfg.Validate()
		require.Empty(t, errs)
	})
}
