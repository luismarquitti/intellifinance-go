package bootstrap

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/luismarquitti/intellifinance-go/internal/config"
)

type personalDatabaseReader interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

// VerifyPersonalDatabase reads a provisioned, immutable identity marker and
// validates role capabilities. It never creates schema or repairs identities.
func VerifyPersonalDatabase(ctx context.Context, db personalDatabaseReader, cfg *config.Config) error {
	u, err := url.Parse(cfg.DatabaseURL)
	if err != nil {
		return errors.New("invalid personal database configuration")
	}
	kind := "personal"
	if strings.HasSuffix(u.Path, "_qa") {
		kind = "qa"
	}
	var allowed bool
	err = db.QueryRow(ctx, `SELECT current_database()=$1 AND current_user='personal_app'
 AND NOT r.rolsuper AND NOT r.rolbypassrls AND NOT r.rolcreatedb AND NOT r.rolcreaterole
 AND EXISTS(SELECT 1 FROM personal_instance_guard g JOIN tenant_members m ON m.tenant_id=g.tenant_id AND m.user_id=g.user_id JOIN users u ON u.id=m.user_id
 WHERE g.singleton AND g.tenant_id=$2 AND g.user_id=$3 AND g.kind=$4 AND u.is_active)
 AND NOT has_table_privilege(current_user,'users','DELETE')
 AND NOT has_table_privilege(current_user,'transactions','TRUNCATE')
 FROM pg_roles r WHERE r.rolname=current_user`, strings.TrimPrefix(u.Path, "/"), cfg.PersonalTenantID, cfg.PersonalUserID, kind).Scan(&allowed)
	if err != nil || !allowed {
		return errors.New("personal database identity, role or protection proof failed")
	}
	return nil
}
