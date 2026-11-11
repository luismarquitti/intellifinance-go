package main_test

import (
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigrateCLI_HelpAndStatus(t *testing.T) {
	// Test help output
	cmd := exec.Command("go", "run", "./main.go")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err)
	require.Contains(t, string(out), "IntelliFinance Migration Engine")
	require.Contains(t, string(out), "-status")
	require.Contains(t, string(out), "-up")

	// Test status if DATABASE_URL is set
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = os.Getenv("TEST_DATABASE_URL")
	}
	if dbURL != "" {
		statusCmd := exec.Command("go", "run", "./main.go", "-status")
		statusCmd.Env = append(os.Environ(), "DATABASE_URL="+dbURL)
		statusOut, statusErr := statusCmd.CombinedOutput()
		require.NoError(t, statusErr, "status output: %s", string(statusOut))
		require.Contains(t, string(statusOut), "VERSION")
		require.Contains(t, string(statusOut), "APPLIED")
	}
}
