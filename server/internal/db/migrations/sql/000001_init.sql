-- Migration: 000001_init
-- Description: Initial schema — users, accounts, domains, backups, domain_profiles, propagation_checks

CREATE TABLE IF NOT EXISTS users (
  id BIGSERIAL PRIMARY KEY,
  email VARCHAR(255) NOT NULL UNIQUE,
  role VARCHAR(32) NOT NULL DEFAULT 'viewer',
  oauth_provider VARCHAR(32) NOT NULL DEFAULT 'dev',
  oauth_subject VARCHAR(255) NOT NULL DEFAULT '',
  oauth_info JSONB NOT NULL DEFAULT '{}'::jsonb,
  token_version INTEGER NOT NULL DEFAULT 1,
  primary_org_id INTEGER NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS organizations (
  id SERIAL PRIMARY KEY,
  name VARCHAR(255) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS org_members (
  id SERIAL PRIMARY KEY,
  org_id INTEGER NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role VARCHAR(32) NOT NULL DEFAULT 'viewer',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(org_id, user_id)
);

CREATE TABLE IF NOT EXISTS accounts (
  id BIGSERIAL PRIMARY KEY,
  org_id INTEGER NOT NULL DEFAULT 0,
  user_id BIGINT NOT NULL,
  name VARCHAR(255) NOT NULL,
  provider VARCHAR(64) NOT NULL,
  encrypted_config JSONB NOT NULL DEFAULT '{}'::jsonb,
  expires_at TIMESTAMPTZ,
  last_checked_at TIMESTAMPTZ,
  last_rotated_at TIMESTAMPTZ,
  last_validation_error TEXT NOT NULL DEFAULT '',
  credential_status VARCHAR(32) NOT NULL DEFAULT 'unknown',
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS domains (
  id BIGSERIAL PRIMARY KEY,
  account_id BIGINT NOT NULL,
  name VARCHAR(255) NOT NULL,
  provider_zone_id VARCHAR(255) NOT NULL,
  is_starred BOOLEAN NOT NULL DEFAULT FALSE,
  is_archived BOOLEAN NOT NULL DEFAULT FALSE,
  archived_at TIMESTAMPTZ,
  tags JSONB NOT NULL DEFAULT '{}'::jsonb,
  last_synced_at TIMESTAMPTZ,
  last_propagation_status JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS backups (
  id BIGSERIAL PRIMARY KEY,
  domain_id BIGINT NOT NULL,
  triggered_by_user_id BIGINT NOT NULL,
  reason VARCHAR(255) NOT NULL,
  content JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS domain_profiles (
  id SERIAL PRIMARY KEY,
  domain_id BIGINT NOT NULL UNIQUE,
  description TEXT NOT NULL DEFAULT '',
  attachment_urls JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS propagation_checks (
  id BIGSERIAL PRIMARY KEY,
  domain_id BIGINT NOT NULL,
  triggered_by_user_id BIGINT NOT NULL,
  fqdn VARCHAR(255) NOT NULL,
  record JSONB NOT NULL DEFAULT '{}'::jsonb,
  overall_status VARCHAR(32) NOT NULL,
  summary VARCHAR(255) NOT NULL,
  matched_count INTEGER NOT NULL DEFAULT 0,
  failed_count INTEGER NOT NULL DEFAULT 0,
  pending_count INTEGER NOT NULL DEFAULT 0,
  total_resolvers INTEGER NOT NULL DEFAULT 0,
  results JSONB NOT NULL DEFAULT '{}'::jsonb,
  checked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS reminder_acks (
  id SERIAL PRIMARY KEY,
  user_id INTEGER NOT NULL,
  account_id INTEGER NOT NULL,
  handled_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(user_id, account_id)
);
