package db

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"gorm.io/gorm"
)

//go:embed migrations/sql/*.sql
var migrationsFS embed.FS

// Applied migration record stored in the DB.
type schemaMigration struct {
	Version uint
	Name    string
	AppliedAt string `gorm:"column:applied_at;default:NOW()"`
}

func (schemaMigration) TableName() string { return "schema_migrations" }

// RunMigrations executes all pending SQL migrations in lexical order.
func RunMigrations(db *gorm.DB) error {
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, name VARCHAR(255) NOT NULL, applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`).Error; err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	entries, err := migrationsFS.ReadDir("migrations/sql")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	pending, err := pendingMigrations(db, entries)
	if err != nil {
		return fmt.Errorf("list pending migrations: %w", err)
	}
	if len(pending) == 0 {
		return nil
	}

	for _, m := range pending {
		sqlBytes, err := migrationsFS.ReadFile("migrations/sql/" + m.filename)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", m.filename, err)
		}
		statements := splitStatements(string(sqlBytes))
		for _, stmt := range statements {
			trimmed := strings.TrimSpace(stmt)
			if trimmed == "" {
				continue
			}
			if err := db.Exec(trimmed).Error; err != nil {
				return fmt.Errorf("apply migration %s: %w", m.filename, err)
			}
		}
		if err := db.Exec("INSERT INTO schema_migrations (version, name) VALUES (?, ?)", m.version, m.filename).Error; err != nil {
			return fmt.Errorf("record migration %d: %w", m.version, err)
		}
	}
	return nil
}

type migrationFile struct {
	version  uint
	filename string
}

func pendingMigrations(db *gorm.DB, entries []fs.DirEntry) ([]migrationFile, error) {
	var applied []schemaMigration
	if err := db.Order("version asc").Find(&applied).Error; err != nil {
		return nil, err
	}
	appliedSet := make(map[uint]bool)
	for _, m := range applied {
		appliedSet[m.Version] = true
	}

	var result []migrationFile
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		ver, err := parseVersion(entry.Name())
		if err != nil {
			continue // skip non-migration files
		}
		if !appliedSet[ver] {
			result = append(result, migrationFile{version: ver, filename: entry.Name()})
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].version < result[j].version })
	return result, nil
}

func parseVersion(name string) (uint, error) {
	// Format: 000001_filename.sql → extract 1
	parts := strings.SplitN(name, "_", 2)
	if len(parts) == 0 {
		return 0, fmt.Errorf("invalid migration name")
	}
	var v uint
	for _, c := range parts[0] {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid version prefix")
		}
		v = v*10 + uint(c-'0')
	}
	return v, nil
}

func splitStatements(sql string) []string {
	var stmts []string
	var current strings.Builder
	inString := false
	escaped := false
	for i := 0; i < len(sql); i++ {
		ch := sql[i]
		if escaped {
			current.WriteByte(ch)
			escaped = false
			continue
		}
		if ch == '\\' {
			current.WriteByte(ch)
			escaped = true
			continue
		}
		if ch == '\'' {
			current.WriteByte(ch)
			inString = !inString
			continue
		}
		if inString {
			current.WriteByte(ch)
			continue
		}
		if ch == ';' {
			stmts = append(stmts, current.String())
			current.Reset()
			continue
		}
		current.WriteByte(ch)
	}
	if s := strings.TrimSpace(current.String()); s != "" {
		stmts = append(stmts, s)
	}
	return stmts
}
