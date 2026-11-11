package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestCanonicalCommandPreviewDoesNotConnectOrRetain(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("synthetic-password"), bcrypt.MinCost)
	require.NoError(t, err)
	for k, v := range map[string]string{"ENVIRONMENT": "personal", "PORT": "8093", "BIND_ADDR": "127.0.0.1", "DATABASE_URL": "postgres://personal_app@127.0.0.1:55434/intellifinance_personal_qa", "PERSONAL_USERNAME": "pilot", "PERSONAL_PASSWORD_HASH": string(hash), "PERSONAL_TENANT_ID": uuid.NewString(), "PERSONAL_USER_ID": uuid.NewString()} {
		t.Setenv(k, v)
	}
	file := filepath.Join(t.TempDir(), "synthetic.csv")
	require.NoError(t, os.WriteFile(file, []byte("ID,Data,Descrição,Valor,Tipo\nSYN-1,01/02/2026,Synthetic,1.00,Despesa\n"), 0o600))
	var out bytes.Buffer
	require.NoError(t, run(context.Background(), []string{"--file", file, "--account", uuid.NewString()}, &out))
	require.Contains(t, out.String(), `"parsed":1`)
	require.Contains(t, out.String(), `"applied":false`)
	require.Error(t, run(context.Background(), []string{"--file", file, "--account", "malformed"}, &out))
	require.Error(t, run(context.Background(), []string{"--file", file, "--account", uuid.NewString(), "--apply"}, &out), "apply requires a private source archive")
}

func TestCanonicalCommandRejectsSymlinkArchiveBeforeDatabase(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("synthetic-password"), bcrypt.MinCost)
	require.NoError(t, err)
	for k, v := range map[string]string{"ENVIRONMENT": "personal", "PORT": "8093", "BIND_ADDR": "127.0.0.1", "DATABASE_URL": "postgres://personal_app@127.0.0.1:55434/intellifinance_personal_qa", "PERSONAL_USERNAME": "pilot", "PERSONAL_PASSWORD_HASH": string(hash), "PERSONAL_TENANT_ID": uuid.NewString(), "PERSONAL_USER_ID": uuid.NewString()} {
		t.Setenv(k, v)
	}
	base := t.TempDir()
	checkout := filepath.Join(base, "checkout")
	require.NoError(t, os.Mkdir(checkout, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(checkout, ".git"), []byte("gitdir: synthetic"), 0600))
	link := filepath.Join(base, "link")
	require.NoError(t, os.Symlink(checkout, link))
	file := filepath.Join(base, "synthetic.csv")
	require.NoError(t, os.WriteFile(file, []byte("ID,Data,Descrição,Valor,Tipo\nSYN-1,01/02/2026,Synthetic,1.00,Despesa\n"), 0600))
	var out bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // The pre-fix implementation must not contact even the QA database.
	err = run(ctx, []string{"--file", file, "--account", uuid.NewString(), "--apply", "--archive-dir", filepath.Join(link, "archive")}, &out)
	require.ErrorContains(t, err, "non-symlink", "shared archive validation must run before any database access")
	_, err = os.Stat(filepath.Join(checkout, "archive"))
	require.True(t, os.IsNotExist(err))
}
