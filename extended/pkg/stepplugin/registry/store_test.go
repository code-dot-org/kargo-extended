package registry

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStorePluginsRequiresInitialSync(t *testing.T) {
	store := NewStore()

	_, err := store.Plugins(t.Context(), "system")
	require.ErrorContains(t, err, "StepPlugin registry has not synced yet")
}
