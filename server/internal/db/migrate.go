package db

import (
	"dns-hub/server/internal/model"

	"gorm.io/gorm"
)

func Migrate(database *gorm.DB) error {
	if err := database.AutoMigrate(
		&model.User{},
		&model.Organization{},
		&model.OrgMember{},
		&model.Account{},
		&model.Domain{},
		&model.Backup{},
		&model.DomainProfile{},
		&model.PropagationCheck{},
		&model.ReminderAck{},
	); err != nil {
		return err
	}
	return patchLegacySchema(database)
}

func patchLegacySchema(database *gorm.DB) error {
	statements := []string{
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS oauth_provider VARCHAR(32) NOT NULL DEFAULT 'dev'`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS oauth_subject VARCHAR(255) NOT NULL DEFAULT ''`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS oauth_info JSONB NOT NULL DEFAULT '{}'::jsonb`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS token_version INTEGER NOT NULL DEFAULT 1`,
		`ALTER TABLE domains ADD COLUMN IF NOT EXISTS is_archived BOOLEAN NOT NULL DEFAULT false`,
		`ALTER TABLE domains ADD COLUMN IF NOT EXISTS archived_at TIMESTAMPTZ`,
		`ALTER TABLE accounts ADD COLUMN IF NOT EXISTS last_rotated_at TIMESTAMPTZ`,
		`ALTER TABLE accounts ADD COLUMN IF NOT EXISTS last_validation_error TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE accounts ADD COLUMN IF NOT EXISTS credential_status VARCHAR(32) NOT NULL DEFAULT 'unknown'`,
		`UPDATE accounts SET credential_status = CASE
			WHEN status = 'active' THEN 'valid'
			WHEN status = 'error' THEN 'invalid'
			ELSE 'unknown'
		END
		WHERE credential_status = '' OR credential_status = 'unknown'`,
		`DO $$
		BEGIN
			IF EXISTS (
				SELECT 1
				FROM information_schema.columns
				WHERE table_name = 'users' AND column_name = 'o_auth_provider'
			) THEN
				ALTER TABLE users ALTER COLUMN o_auth_provider SET DEFAULT 'dev';
				UPDATE users
				SET o_auth_provider = COALESCE(NULLIF(o_auth_provider, ''), oauth_provider, 'dev')
				WHERE o_auth_provider IS NULL OR o_auth_provider = '';
				UPDATE users
				SET oauth_provider = COALESCE(NULLIF(o_auth_provider, ''), oauth_provider)
				WHERE oauth_provider = 'dev';
			END IF;
		END
		$$`,
		`DO $$
		BEGIN
			IF EXISTS (
				SELECT 1
				FROM information_schema.columns
				WHERE table_name = 'users' AND column_name = 'o_auth_subject'
			) THEN
				ALTER TABLE users ALTER COLUMN o_auth_subject SET DEFAULT '';
				UPDATE users
				SET o_auth_subject = COALESCE(NULLIF(o_auth_subject, ''), oauth_subject, email, '')
				WHERE o_auth_subject IS NULL OR o_auth_subject = '';
				UPDATE users
				SET oauth_subject = COALESCE(NULLIF(o_auth_subject, ''), oauth_subject)
				WHERE oauth_subject = '';
			END IF;
		END
		$$`,
		`DO $$
		BEGIN
			IF EXISTS (
				SELECT 1
				FROM information_schema.columns
				WHERE table_name = 'users' AND column_name = 'o_auth_info'
			) THEN
				ALTER TABLE users ALTER COLUMN o_auth_info SET DEFAULT '{}'::jsonb;
				UPDATE users
				SET o_auth_info = COALESCE(o_auth_info, oauth_info, '{}'::jsonb)
				WHERE o_auth_info IS NULL;
				UPDATE users
				SET oauth_info = COALESCE(o_auth_info, oauth_info)
				WHERE oauth_info = '{}'::jsonb;
			END IF;
		END
		$$`,
		`UPDATE users SET oauth_subject = email WHERE oauth_subject = ''`,
		// Multi-tenant: add org_id to accounts and primary_org_id to users
		`ALTER TABLE accounts ADD COLUMN IF NOT EXISTS org_id INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS primary_org_id INTEGER NOT NULL DEFAULT 0`,
		// Migrate existing accounts: set org_id to a default org, creating it first if needed
		`INSERT INTO organizations (id, name, created_at, updated_at)
		 VALUES (1, 'Default Organization', NOW(), NOW())
		 ON CONFLICT DO NOTHING`,
		`UPDATE accounts SET org_id = 1 WHERE org_id = 0 OR org_id IS NULL`,
		`UPDATE users SET primary_org_id = 1 WHERE primary_org_id = 0 OR primary_org_id IS NULL`,
	}
	for _, statement := range statements {
		if err := database.Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}
