package service

import (
	"fmt"
	"strings"

	"dns-hub/server/internal/model"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type SnapshotService struct {
	db *gorm.DB
}

func NewSnapshotService(db *gorm.DB) *SnapshotService {
	return &SnapshotService{db: db}
}

func (s *SnapshotService) Create(domainID, userID uint, reason string, records any) (*model.Backup, error) {
	payload, ok := records.(map[string]any)
	if !ok {
		payload = map[string]any{"records": records}
	}
	backup := model.Backup{
		DomainID:          domainID,
		TriggeredByUserID: userID,
		Reason:            reason,
		Content:           datatypes.JSONMap(payload),
	}
	if err := s.db.Create(&backup).Error; err != nil {
		return nil, fmt.Errorf("create snapshot: %w", err)
	}
	return &backup, nil
}

func (s *SnapshotService) ListByDomain(domainID uint) ([]model.Backup, error) {
	var backups []model.Backup
	if err := s.db.Where("domain_id = ?", domainID).Order("created_at desc").Find(&backups).Error; err != nil {
		return nil, err
	}
	return backups, nil
}

func (s *SnapshotService) ListForUser(userID uint, search string) ([]model.Backup, error) {
	query := s.db.Table("backups").
		Select("backups.*").
		Joins("join domains on domains.id = backups.domain_id").
		Joins("join accounts on accounts.id = domains.account_id").
		Where("accounts.user_id = ?", userID)
	if trimmed := strings.TrimSpace(search); trimmed != "" {
		like := "%" + trimmed + "%"
		query = query.Where("domains.name ILIKE ? OR backups.reason ILIKE ?", like, like)
	}
	var backups []model.Backup
	if err := query.Order("backups.created_at desc").Find(&backups).Error; err != nil {
		return nil, err
	}
	return backups, nil
}

func (s *SnapshotService) GetForUser(userID, backupID uint) (*model.Backup, error) {
	var backup model.Backup
	if err := s.db.Table("backups").
		Select("backups.*").
		Joins("join domains on domains.id = backups.domain_id").
		Joins("join accounts on accounts.id = domains.account_id").
		Where("accounts.user_id = ? AND backups.id = ?", userID, backupID).
		First(&backup).Error; err != nil {
		return nil, err
	}
	return &backup, nil
}

func (s *SnapshotService) LatestByReason(domainID uint, reason string) (*model.Backup, error) {
	var backup model.Backup
	if err := s.db.Where("domain_id = ? AND reason = ?", domainID, reason).Order("created_at desc").First(&backup).Error; err != nil {
		return nil, err
	}
	return &backup, nil
}

func (s *SnapshotService) RecordRestore(domainID, userID uint, sourceBackupID uint, records any) (*model.Backup, error) {
	return s.Create(domainID, userID, fmt.Sprintf("restore_from_backup_%d", sourceBackupID), map[string]any{
		"restoredFromBackupId": sourceBackupID,
		"records":              records,
	})
}
