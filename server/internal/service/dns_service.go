package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"dns-hub/server/internal/model"
	"dns-hub/server/internal/provider"
	_ "dns-hub/server/internal/provider/alidns"
	_ "dns-hub/server/internal/provider/aws"
	_ "dns-hub/server/internal/provider/cloudflare"
	_ "dns-hub/server/internal/provider/digitalocean"
	_ "dns-hub/server/internal/provider/dnspod"
	_ "dns-hub/server/internal/provider/gcp"
	_ "dns-hub/server/internal/provider/hetzner"
	_ "dns-hub/server/internal/provider/huawei"
	_ "dns-hub/server/internal/provider/mock"
	_ "dns-hub/server/internal/provider/mocklike"
	_ "dns-hub/server/internal/provider/namecheap"
	_ "dns-hub/server/internal/provider/vultr"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type DNSService struct {
	db           *gorm.DB
	crypto       *CryptoService
	snapshots    *SnapshotService
	propagation  *PropagationService
	reminderSvc  *ReminderService
}

type AccountInput struct {
	Name      string
	Provider  string
	Config    map[string]any
	ExpiresAt *time.Time
	Status    string
}

type AccountView struct {
	ID                  uint       `json:"id"`
	Name                string     `json:"name"`
	Provider            string     `json:"provider"`
	Status              string     `json:"status"`
	CredentialStatus    string     `json:"credentialStatus"`
	ExpiresAt           *time.Time `json:"expiresAt"`
	LastCheckedAt       *time.Time `json:"lastCheckedAt"`
	LastRotatedAt       *time.Time `json:"lastRotatedAt"`
	LastValidationError string     `json:"lastValidationError"`
	DomainCount         int64      `json:"domainCount"`
	Reminder            string     `json:"reminder"`
}

type DomainView struct {
	ID                    uint              `json:"id"`
	AccountID             uint              `json:"accountId"`
	AccountName           string            `json:"accountName"`
	Provider              string            `json:"provider"`
	Name                  string            `json:"name"`
	ProviderZoneID        string            `json:"providerZoneId"`
	IsStarred             bool              `json:"isStarred"`
	IsArchived            bool              `json:"isArchived"`
	ArchivedAt            *time.Time        `json:"archivedAt"`
	Tags                  []string          `json:"tags"`
	LastSyncedAt          *time.Time        `json:"lastSyncedAt"`
	LastPropagationStatus datatypes.JSONMap `json:"lastPropagationStatus"`
	CreatedAt             time.Time         `json:"createdAt"`
	UpdatedAt             time.Time         `json:"updatedAt"`
}

type BackupListItem struct {
	model.Backup
	DomainName   string `json:"domainName"`
	AccountName  string `json:"accountName"`
	Provider     string `json:"provider"`
	RecordCount  int    `json:"recordCount"`
	RestoreLabel string `json:"restoreLabel,omitempty"`
}

type DomainListOptions struct {
	Search          string
	IncludeArchived bool
}

type DomainProfileView struct {
	ID             uint      `json:"id"`
	DomainID       uint      `json:"domainId"`
	Description    string    `json:"description"`
	AttachmentURLs []string  `json:"attachmentUrls"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type DashboardSummary struct {
	Accounts         int64 `json:"accounts"`
	Domains          int64 `json:"domains"`
	StarredDomains   int64 `json:"starredDomains"`
	ExpiringAccounts int   `json:"expiringAccounts"`
}

type ProviderCatalog struct {
	Items []provider.Descriptor `json:"items"`
}

func (s *DNSService) ListProviderCatalog() []provider.Descriptor {
	return provider.RegisteredDescriptors()
}

func NewDNSService(db *gorm.DB, crypto *CryptoService, snapshots *SnapshotService, propagation *PropagationService, reminderSvc *ReminderService) *DNSService {
	return &DNSService{db: db, crypto: crypto, snapshots: snapshots, propagation: propagation, reminderSvc: reminderSvc}
}

func (s *DNSService) ListAccounts(userID uint) ([]AccountView, error) {
	orgID, err := s.getUserOrgID(userID)
	if err != nil {
		return nil, err
	}
	var accounts []model.Account
	if err := s.db.Where("org_id = ?", orgID).Order("created_at desc").Find(&accounts).Error; err != nil {
		return nil, err
	}
	views := make([]AccountView, 0, len(accounts))
	for _, account := range accounts {
		var domainCount int64
		if err := s.db.Model(&model.Domain{}).Where("account_id = ?", account.ID).Count(&domainCount).Error; err != nil {
			return nil, err
		}
		views = append(views, AccountView{
			ID:                  account.ID,
			Name:                account.Name,
			Provider:            account.Provider,
			Status:              account.Status,
			CredentialStatus:    accountCredentialStatus(account),
			ExpiresAt:           account.ExpiresAt,
			LastCheckedAt:       account.LastCheckedAt,
			LastRotatedAt:       account.LastRotatedAt,
			LastValidationError: account.LastValidationError,
			DomainCount:         domainCount,
			Reminder:            severityForExpiry(account.ExpiresAt),
		})
	}
	return views, nil
}

func (s *DNSService) getUserOrgID(userID uint) (uint, error) {
	var user model.User
	if err := s.db.First(&user, userID).Error; err != nil {
		return 0, err
	}
	if user.PrimaryOrgID == 0 {
		// Fallback: use userID for legacy accounts
		return userID, nil
	}
	return user.PrimaryOrgID, nil
}

func (s *DNSService) CreateAccount(ctx context.Context, userID uint, input AccountInput) (*model.Account, error) {
	if strings.TrimSpace(input.Name) == "" {
		return nil, fmt.Errorf("account name is required")
	}
	providerName := strings.ToLower(strings.TrimSpace(input.Provider))
	if providerName == "" {
		return nil, fmt.Errorf("provider is required")
	}
	if input.Config == nil {
		input.Config = map[string]any{}
	}
	orgID, err := s.getUserOrgID(userID)
	if err != nil {
		return nil, fmt.Errorf("get user org: %w", err)
	}
	account := model.Account{
		OrgID:               orgID,
		UserID:              userID,
		Name:                strings.TrimSpace(input.Name),
		Provider:            providerName,
		ExpiresAt:           input.ExpiresAt,
		Status:              firstNonEmpty(strings.TrimSpace(input.Status), "active"),
		CredentialStatus:    "pending",
		LastValidationError: "",
		EncryptedConfig:     datatypes.JSONMap{},
	}
	config, err := s.crypto.EncryptConfig(input.Config, s.accountAAD(account))
	if err != nil {
		return nil, err
	}
	account.EncryptedConfig = config
	if err := s.db.Create(&account).Error; err != nil {
		return nil, err
	}
	if _, err := s.ValidateAndSyncAccount(ctx, userID, account.ID); err != nil {
		return &account, nil
	}
	return &account, nil
}

func (s *DNSService) UpdateAccount(ctx context.Context, userID, accountID uint, input AccountInput) (*model.Account, error) {
	account, err := s.getAccountForUser(accountID, userID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(input.Name) != "" {
		account.Name = strings.TrimSpace(input.Name)
	}
	if strings.TrimSpace(input.Status) != "" {
		account.Status = strings.TrimSpace(input.Status)
	}
	account.ExpiresAt = input.ExpiresAt
	if input.Config != nil {
		config, err := s.crypto.EncryptConfig(input.Config, s.accountAAD(*account))
		if err != nil {
			return nil, err
		}
		account.EncryptedConfig = config
		account.CredentialStatus = "pending"
		account.LastValidationError = ""
	}
	if err := s.db.Save(account).Error; err != nil {
		return nil, err
	}
	if _, err := s.ValidateAndSyncAccount(ctx, userID, account.ID); err != nil {
		return account, nil
	}
	return account, nil
}

func (s *DNSService) ValidateAndSyncAccount(ctx context.Context, userID, accountID uint) (*provider.ValidationResult, error) {
	account, err := s.getAccountForUser(accountID, userID)
	if err != nil {
		return nil, err
	}
	adapter, err := s.providerForAccount(*account)
	if err != nil {
		s.markAccountValidationFailure(account, err)
		return nil, err
	}
	result, err := adapter.Validate(ctx)
	if err != nil {
		s.markAccountValidationFailure(account, err)
		return nil, err
	}
	now := valueOrNow(result.CheckedAt)
	account.Status = "active"
	account.CredentialStatus = "valid"
	account.LastValidationError = ""
	account.LastCheckedAt = &now
	if err := s.db.Save(account).Error; err != nil {
		return nil, err
	}
	if err := s.syncDomains(ctx, *account, adapter); err != nil {
		return result, err
	}
	return result, nil
}

func (s *DNSService) RotateAccountCredentials(ctx context.Context, userID, accountID uint, input AccountInput) (*model.Account, *provider.ValidationResult, error) {
	account, err := s.getAccountForUser(accountID, userID)
	if err != nil {
		return nil, nil, err
	}
	if input.Config == nil {
		return nil, nil, fmt.Errorf("config is required")
	}
	if strings.TrimSpace(input.Name) != "" {
		account.Name = strings.TrimSpace(input.Name)
	}
	if strings.TrimSpace(input.Status) != "" {
		account.Status = strings.TrimSpace(input.Status)
	}
	account.ExpiresAt = input.ExpiresAt
	config, err := s.crypto.EncryptConfig(input.Config, s.accountAAD(*account))
	if err != nil {
		return nil, nil, err
	}
	now := time.Now().UTC()
	account.EncryptedConfig = config
	account.LastRotatedAt = &now
	account.CredentialStatus = "pending"
	account.LastValidationError = ""
	if err := s.db.Save(account).Error; err != nil {
		return nil, nil, err
	}
	result, err := s.ValidateAndSyncAccount(ctx, userID, account.ID)
	if err != nil {
		return account, nil, err
	}
	if err := s.db.First(account, account.ID).Error; err != nil {
		return nil, result, err
	}
	return account, result, nil
}

func (s *DNSService) markAccountValidationFailure(account *model.Account, validationErr error) {
	now := time.Now().UTC()
	account.Status = "error"
	account.CredentialStatus = "invalid"
	account.LastValidationError = validationErr.Error()
	account.LastCheckedAt = &now
	_ = s.db.Save(account).Error
}

// ReactivateAccount re-validates the account credentials and syncs domains.
// If credentials were renewed manually, this marks the account as valid again.
func (s *DNSService) ReactivateAccount(ctx context.Context, userID, accountID uint) error {
	account, err := s.getAccountForUser(accountID, userID)
	if err != nil {
		return err
	}
	// Only reactivate accounts that are currently in an error state
	if account.CredentialStatus != "invalid" && account.Status != "error" {
		return nil
	}
	_, err = s.ValidateAndSyncAccount(ctx, userID, account.ID)
	return err
}

func valueOrNow(value time.Time) time.Time {
	if value.IsZero() {
		return time.Now().UTC()
	}
	return value.UTC()
}

func (s *DNSService) ListDomains(userID uint, search string) ([]DomainView, error) {
	return s.ListDomainsWithOptions(userID, DomainListOptions{Search: search})
}

func (s *DNSService) ListDomainsWithOptions(userID uint, options DomainListOptions) ([]DomainView, error) {
	orgID, err := s.getUserOrgID(userID)
	if err != nil {
		return nil, err
	}
	type row struct {
		model.Domain
		AccountName string `json:"accountName"`
		Provider    string `json:"provider"`
	}
	query := s.db.Table("domains").
		Select("domains.*, accounts.name as account_name, accounts.provider").
		Joins("join accounts on accounts.id = domains.account_id").
		Where("accounts.org_id = ?", orgID)
	if !options.IncludeArchived {
		query = query.Where("domains.is_archived = ?", false)
	}
	if trimmed := strings.TrimSpace(options.Search); trimmed != "" {
		like := "%" + trimmed + "%"
		query = query.Where("domains.name ILIKE ? OR accounts.name ILIKE ?", like, like)
	}
	var rows []row
	if err := query.Order("domains.is_archived asc, domains.is_starred desc, domains.name asc").Scan(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]DomainView, 0, len(rows))
	for _, item := range rows {
		items = append(items, DomainView{
			ID:                    item.ID,
			AccountID:             item.AccountID,
			AccountName:           item.AccountName,
			Provider:              item.Provider,
			Name:                  item.Name,
			ProviderZoneID:        item.ProviderZoneID,
			IsStarred:             item.IsStarred,
			IsArchived:            item.IsArchived,
			ArchivedAt:            item.ArchivedAt,
			Tags:                  stringsFromJSONMap(item.Tags),
			LastSyncedAt:          item.LastSyncedAt,
			LastPropagationStatus: ensureJSONMap(item.LastPropagationStatus),
			CreatedAt:             item.CreatedAt,
			UpdatedAt:             item.UpdatedAt,
		})
	}
	return items, nil
}

func (s *DNSService) SetDomainArchived(userID, domainID uint, archived bool) (*model.Domain, error) {
	domain, _, err := s.getDomainForUser(domainID, userID)
	if err != nil {
		return nil, err
	}
	domain.IsArchived = archived
	if archived {
		now := time.Now().UTC()
		domain.ArchivedAt = &now
	} else {
		domain.ArchivedAt = nil
	}
	if err := s.db.Save(domain).Error; err != nil {
		return nil, err
	}
	return domain, nil
}

func (s *DNSService) ToggleDomainStar(userID, domainID uint) (*model.Domain, error) {
	domain, _, err := s.getDomainForUser(domainID, userID)
	if err != nil {
		return nil, err
	}
	domain.IsStarred = !domain.IsStarred
	if err := s.db.Save(domain).Error; err != nil {
		return nil, err
	}
	return domain, nil
}

func (s *DNSService) UpdateDomainTags(userID, domainID uint, tags []string) (*model.Domain, error) {
	domain, _, err := s.getDomainForUser(domainID, userID)
	if err != nil {
		return nil, err
	}
	domain.Tags = stringsToJSONMap(tags)
	if err := s.db.Save(domain).Error; err != nil {
		return nil, err
	}
	return domain, nil
}

func (s *DNSService) ListRecords(ctx context.Context, userID, domainID uint) ([]provider.DNSRecord, error) {
	domain, account, err := s.getDomainForUser(domainID, userID)
	if err != nil {
		return nil, err
	}
	adapter, err := s.providerForAccount(*account)
	if err != nil {
		return nil, err
	}
	return adapter.ListRecords(ctx, domain.ProviderZoneID)
}

func (s *DNSService) UpsertRecord(ctx context.Context, userID, domainID uint, input provider.RecordMutation) (*provider.DNSRecord, *model.Backup, datatypes.JSONMap, error) {
	domain, account, err := s.getDomainForUser(domainID, userID)
	if err != nil {
		return nil, nil, nil, err
	}
	adapter, err := s.providerForAccount(*account)
	if err != nil {
		return nil, nil, nil, err
	}
	records, err := adapter.ListRecords(ctx, domain.ProviderZoneID)
	if err != nil {
		return nil, nil, nil, err
	}
	backup, err := s.snapshots.Create(domain.ID, userID, "before_record_upsert", map[string]any{"records": records})
	if err != nil {
		return nil, nil, nil, err
	}
	record, err := adapter.UpsertRecord(ctx, domain.ProviderZoneID, input)
	if err != nil {
		return nil, backup, nil, err
	}
	if err := s.persistProviderConfig(account, adapter); err != nil {
		return nil, backup, nil, err
	}
	status, err := s.propagation.CheckAndPersist(ctx, domain.ID, userID, domain.Name, *record)
	return record, backup, status, err
}

func (s *DNSService) DeleteRecord(ctx context.Context, userID, domainID uint, recordID string) (*model.Backup, error) {
	domain, account, err := s.getDomainForUser(domainID, userID)
	if err != nil {
		return nil, err
	}
	adapter, err := s.providerForAccount(*account)
	if err != nil {
		return nil, err
	}
	records, err := adapter.ListRecords(ctx, domain.ProviderZoneID)
	if err != nil {
		return nil, err
	}
	backup, err := s.snapshots.Create(domain.ID, userID, "before_record_delete", map[string]any{"records": records})
	if err != nil {
		return nil, err
	}
	if err := adapter.DeleteRecord(ctx, domain.ProviderZoneID, recordID); err != nil {
		return backup, err
	}
	if err := s.persistProviderConfig(account, adapter); err != nil {
		return backup, err
	}
	return backup, nil
}

func (s *DNSService) TriggerPropagationCheck(ctx context.Context, userID, domainID uint, input provider.RecordMutation) (datatypes.JSONMap, error) {
	domain, _, err := s.getDomainForUser(domainID, userID)
	if err != nil {
		return nil, err
	}
	record := provider.DNSRecord{
		ID:       input.ID,
		Type:     input.Type,
		Name:     input.Name,
		Content:  input.Content,
		TTL:      input.TTL,
		Priority: input.Priority,
		Proxied:  input.Proxied,
		Comment:  input.Comment,
	}
	return s.propagation.CheckAndPersist(ctx, domain.ID, userID, domain.Name, record)
}

func (s *DNSService) TriggerPropagationCheckWithOptions(ctx context.Context, userID, domainID uint, input provider.RecordMutation, opts WatchOptions, watch bool) (datatypes.JSONMap, error) {
	domain, _, err := s.getDomainForUser(domainID, userID)
	if err != nil {
		return nil, err
	}
	record := provider.DNSRecord{
		ID:       input.ID,
		Type:     input.Type,
		Name:     input.Name,
		Content:  input.Content,
		TTL:      input.TTL,
		Priority: input.Priority,
		Proxied:  input.Proxied,
		Comment:  input.Comment,
	}
	if watch {
		return s.propagation.Watch(ctx, domain.ID, userID, domain.Name, record, opts)
	}
	return s.propagation.CheckAndPersist(ctx, domain.ID, userID, domain.Name, record)
}

func (s *DNSService) ListPropagationHistory(userID, domainID uint) ([]PropagationHistoryItem, error) {
	if domainID != 0 {
		if _, _, err := s.getDomainForUser(domainID, userID); err != nil {
			return nil, err
		}
	}
	return s.propagation.ListHistory(userID, domainID)
}

func (s *DNSService) ListBackups(userID, domainID uint) ([]model.Backup, error) {
	domain, _, err := s.getDomainForUser(domainID, userID)
	if err != nil {
		return nil, err
	}
	return s.snapshots.ListByDomain(domain.ID)
}

func (s *DNSService) ListAllBackups(userID uint, search string) ([]BackupListItem, error) {
	type row struct {
		model.Backup
		DomainName  string `json:"domainName"`
		AccountName string `json:"accountName"`
		Provider    string `json:"provider"`
	}
	query := s.db.Table("backups").
		Select("backups.*, domains.name as domain_name, accounts.name as account_name, accounts.provider").
		Joins("join domains on domains.id = backups.domain_id").
		Joins("join accounts on accounts.id = domains.account_id").
		Where("accounts.user_id = ?", userID)
	if trimmed := strings.TrimSpace(search); trimmed != "" {
		like := "%" + trimmed + "%"
		query = query.Where("domains.name ILIKE ? OR accounts.name ILIKE ? OR backups.reason ILIKE ?", like, like, like)
	}
	var rows []row
	if err := query.Order("backups.created_at desc").Scan(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]BackupListItem, 0, len(rows))
	for _, item := range rows {
		recordCount := countBackupRecords(item.Content)
		restoreLabel := ""
		if value, ok := item.Content["restoredFromBackupId"]; ok {
			restoreLabel = fmt.Sprintf("来自快照 #%v 的恢复操作", value)
		}
		items = append(items, BackupListItem{
			Backup:       item.Backup,
			DomainName:   item.DomainName,
			AccountName:  item.AccountName,
			Provider:     item.Provider,
			RecordCount:  recordCount,
			RestoreLabel: restoreLabel,
		})
	}
	return items, nil
}

func (s *DNSService) ExportBackup(userID, backupID uint) ([]byte, string, error) {
	backup, err := s.snapshots.GetForUser(userID, backupID)
	if err != nil {
		return nil, "", err
	}
	type exportPayload struct {
		ID                uint              `json:"id"`
		DomainID          uint              `json:"domainId"`
		TriggeredByUserID uint              `json:"triggeredByUserId"`
		Reason            string            `json:"reason"`
		Content           datatypes.JSONMap `json:"content"`
		CreatedAt         time.Time         `json:"createdAt"`
		ExportedAt        time.Time         `json:"exportedAt"`
		Version           string            `json:"version"`
		Format            string            `json:"format"`
		Source            string            `json:"source"`
	}
	payload, err := json.MarshalIndent(exportPayload{
		ID:                backup.ID,
		DomainID:          backup.DomainID,
		TriggeredByUserID: backup.TriggeredByUserID,
		Reason:            backup.Reason,
		Content:           backup.Content,
		CreatedAt:         backup.CreatedAt,
		ExportedAt:        time.Now().UTC(),
		Version:           "v1",
		Format:            "dns-hub-backup",
		Source:            "dns-hub",
	}, "", "  ")
	if err != nil {
		return nil, "", fmt.Errorf("marshal backup export: %w", err)
	}
	filename := fmt.Sprintf("backup-%d-%s.json", backup.ID, backup.CreatedAt.UTC().Format("20060102T150405Z"))
	return payload, filename, nil
}

func (s *DNSService) RestoreBackup(ctx context.Context, userID, backupID uint) (*model.Backup, error) {
	backup, err := s.snapshots.GetForUser(userID, backupID)
	if err != nil {
		return nil, err
	}
	domain, account, err := s.getDomainForUser(backup.DomainID, userID)
	if err != nil {
		return nil, err
	}
	adapter, err := s.providerForAccount(*account)
	if err != nil {
		return nil, err
	}
	currentRecords, err := adapter.ListRecords(ctx, domain.ProviderZoneID)
	if err != nil {
		return nil, err
	}
	if _, err := s.snapshots.Create(domain.ID, userID, fmt.Sprintf("before_restore_from_backup_%d", backup.ID), map[string]any{"records": currentRecords}); err != nil {
		return nil, err
	}
	targetRecords := recordsFromBackupContent(backup.Content)
	if err := s.replaceZoneRecords(ctx, adapter, domain.ProviderZoneID, currentRecords, targetRecords); err != nil {
		return nil, err
	}
	if err := s.persistProviderConfig(account, adapter); err != nil {
		return nil, err
	}
	restoredBackup, err := s.snapshots.RecordRestore(domain.ID, userID, backup.ID, targetRecords)
	if err != nil {
		return nil, err
	}
	if len(targetRecords) > 0 {
		_, _ = s.propagation.CheckAndPersist(ctx, domain.ID, userID, domain.Name, targetRecords[0])
	}
	return restoredBackup, nil
}

func (s *DNSService) replaceZoneRecords(ctx context.Context, adapter provider.DNSProvider, zoneID string, currentRecords []provider.DNSRecord, targetRecords []provider.DNSRecord) error {
	currentByID := make(map[string]provider.DNSRecord, len(currentRecords))
	for _, item := range currentRecords {
		currentByID[item.ID] = item
	}
	targetByID := make(map[string]provider.DNSRecord, len(targetRecords))
	for _, item := range targetRecords {
		targetByID[item.ID] = item
	}
	for _, item := range currentRecords {
		if item.ID == "" {
			continue
		}
		if _, ok := targetByID[item.ID]; ok {
			continue
		}
		if err := adapter.DeleteRecord(ctx, zoneID, item.ID); err != nil {
			return err
		}
	}
	for _, item := range targetRecords {
		mutation := provider.RecordMutation{
			ID:       item.ID,
			Type:     item.Type,
			Name:     item.Name,
			Content:  item.Content,
			TTL:      item.TTL,
			Priority: item.Priority,
			Proxied:  item.Proxied,
			Comment:  item.Comment,
		}
		if _, exists := currentByID[item.ID]; !exists {
			mutation.ID = ""
		}
		if _, err := adapter.UpsertRecord(ctx, zoneID, mutation); err != nil {
			return err
		}
	}
	return nil
}

func recordsFromBackupContent(content datatypes.JSONMap) []provider.DNSRecord {
	raw, ok := content["records"]
	if !ok || raw == nil {
		return []provider.DNSRecord{}
	}
	if items, ok := raw.([]provider.DNSRecord); ok {
		return items
	}
	if items, ok := raw.([]any); ok {
		result := make([]provider.DNSRecord, 0, len(items))
		for _, item := range items {
			mapped, ok := item.(map[string]any)
			if !ok {
				continue
			}
			record := provider.DNSRecord{
				ID:      stringValue(mapped["id"]),
				Type:    stringValue(mapped["type"]),
				Name:    stringValue(mapped["name"]),
				Content: stringValue(mapped["content"]),
				TTL:     intValue(mapped["ttl"], 300),
				Comment: stringValue(mapped["comment"]),
			}
			if value, ok := mapped["priority"]; ok {
				priority := intValue(value, 10)
				record.Priority = &priority
			}
			if value, ok := mapped["proxied"].(bool); ok {
				record.Proxied = &value
			}
			result = append(result, record)
		}
		return result
	}
	return []provider.DNSRecord{}
}

func countBackupRecords(content datatypes.JSONMap) int {
	return len(recordsFromBackupContent(content))
}

func stringValue(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func intValue(value any, fallback int) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return fallback
	}
}

func (s *DNSService) GetDomainProfile(userID, domainID uint) (*DomainProfileView, error) {
	domain, _, err := s.getDomainForUser(domainID, userID)
	if err != nil {
		return nil, err
	}
	var profile model.DomainProfile
	if err := s.db.Where("domain_id = ?", domain.ID).First(&profile).Error; err != nil {
		if err != gorm.ErrRecordNotFound {
			return nil, err
		}
		profile = model.DomainProfile{DomainID: domain.ID, AttachmentURLs: stringsToJSONMap(nil)}
		if err := s.db.Create(&profile).Error; err != nil {
			return nil, err
		}
	}
	return &DomainProfileView{
		ID:             profile.ID,
		DomainID:       profile.DomainID,
		Description:    profile.Description,
		AttachmentURLs: stringsFromJSONMap(profile.AttachmentURLs),
		CreatedAt:      profile.CreatedAt,
		UpdatedAt:      profile.UpdatedAt,
	}, nil
}

func (s *DNSService) UpdateDomainProfile(userID, domainID uint, description string, attachmentURLs []string) (*DomainProfileView, error) {
	domain, _, err := s.getDomainForUser(domainID, userID)
	if err != nil {
		return nil, err
	}
	var profile model.DomainProfile
	err = s.db.Where("domain_id = ?", domain.ID).First(&profile).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	profile.DomainID = domain.ID
	profile.Description = description
	profile.AttachmentURLs = stringsToJSONMap(attachmentURLs)
	if err == gorm.ErrRecordNotFound {
		if err := s.db.Create(&profile).Error; err != nil {
			return nil, err
		}
	} else {
		if err := s.db.Save(&profile).Error; err != nil {
			return nil, err
		}
	}
	return s.GetDomainProfile(userID, domainID)
}

func (s *DNSService) GetDashboardSummary(userID uint) (*DashboardSummary, error) {
	orgID, err := s.getUserOrgID(userID)
	if err != nil {
		return nil, err
	}
	var summary DashboardSummary
	if err := s.db.Model(&model.Account{}).Where("org_id = ?", orgID).Count(&summary.Accounts).Error; err != nil {
		return nil, err
	}
	if err := s.db.Table("domains").Joins("join accounts on accounts.id = domains.account_id").Where("accounts.org_id = ?", orgID).Count(&summary.Domains).Error; err != nil {
		return nil, err
	}
	if err := s.db.Table("domains").Joins("join accounts on accounts.id = domains.account_id").Where("accounts.org_id = ? AND domains.is_starred = ?", orgID, true).Count(&summary.StarredDomains).Error; err != nil {
		return nil, err
	}
	reminders, err := s.ListReminders(userID)
	if err != nil {
		return nil, err
	}
	summary.ExpiringAccounts = len(reminders)
	return &summary, nil
}

func (s *DNSService) ListReminders(userID uint) ([]Reminder, error) {
	orgID, err := s.getUserOrgID(userID)
	if err != nil {
		return nil, err
	}
	var accounts []model.Account
	if err := s.db.Where("org_id = ? AND expires_at IS NOT NULL", orgID).Find(&accounts).Error; err != nil {
		return nil, err
	}
	acks, err := s.reminderSvc.GetReminderAcks(userID)
	if err != nil {
		return nil, err
	}
	items := make([]Reminder, 0)
	now := time.Now().UTC()
	for _, account := range accounts {
		severity := severityForExpiry(account.ExpiresAt)
		if severity == "" || account.ExpiresAt == nil {
			continue
		}
		ack := acks[account.ID]
		items = append(items, Reminder{
			AccountID: account.ID,
			Name:      account.Name,
			Provider:  account.Provider,
			UserID:    account.UserID,
			ExpiresAt: account.ExpiresAt,
			Severity:  severity,
			DaysLeft:  int(account.ExpiresAt.Sub(now).Hours() / 24),
			Handled:   ack.Handled,
			HandledAt: ack.HandledAt,
		})
	}
	return items, nil
}

func (s *DNSService) SetReminderHandled(userID, accountID uint, handled bool) error {
	return s.reminderSvc.SetReminderHandled(userID, accountID, handled)
}

func (s *DNSService) getAccountForUser(accountID, userID uint) (*model.Account, error) {
	var account model.Account
	if err := s.db.Where("id = ? AND user_id = ?", accountID, userID).First(&account).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

func (s *DNSService) getDomainForUser(domainID, userID uint) (*model.Domain, *model.Account, error) {
	var domain model.Domain
	if err := s.db.First(&domain, domainID).Error; err != nil {
		return nil, nil, err
	}
	account, err := s.getAccountForUser(domain.AccountID, userID)
	if err != nil {
		return nil, nil, err
	}
	return &domain, account, nil
}

func (s *DNSService) syncDomains(ctx context.Context, account model.Account, adapter provider.DNSProvider) error {
	domains, err := adapter.ListDomains(ctx)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, item := range domains {
		var domain model.Domain
		err := s.db.Where("account_id = ? AND provider_zone_id = ?", account.ID, item.ZoneID).First(&domain).Error
		if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}
		domain.AccountID = account.ID
		domain.Name = item.Name
		domain.ProviderZoneID = item.ZoneID
		domain.LastSyncedAt = &now
		if domain.Tags == nil {
			domain.Tags = stringsToJSONMap(nil)
		}
		if domain.LastPropagationStatus == nil {
			domain.LastPropagationStatus = datatypes.JSONMap{}
		}
		if err == gorm.ErrRecordNotFound {
			if err := s.db.Create(&domain).Error; err != nil {
				return err
			}
			continue
		}
		if err := s.db.Save(&domain).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *DNSService) providerForAccount(account model.Account) (provider.DNSProvider, error) {
	decrypted, err := s.crypto.DecryptConfig(account.EncryptedConfig, s.accountAAD(account))
	if err != nil {
		return nil, err
	}
	return provider.New(account.Provider, decrypted)
}

func (s *DNSService) persistProviderConfig(account *model.Account, adapter provider.DNSProvider) error {
	config, err := s.crypto.EncryptConfig(adapter.ExportConfig(), s.accountAAD(*account))
	if err != nil {
		return err
	}
	account.EncryptedConfig = config
	return s.db.Model(account).Update("encrypted_config", config).Error
}

func (s *DNSService) accountAAD(account model.Account) string {
	return fmt.Sprintf("provider:%s:user:%d", account.Provider, account.UserID)
}

func stringsToJSONMap(items []string) datatypes.JSONMap {
	values := make([]any, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		values = append(values, trimmed)
	}
	return datatypes.JSONMap{"items": values}
}

func stringsFromJSONMap(input datatypes.JSONMap) []string {
	if input == nil {
		return []string{}
	}
	raw, ok := input["items"]
	if !ok {
		return []string{}
	}
	if items, ok := raw.([]any); ok {
		values := make([]string, 0, len(items))
		for _, item := range items {
			if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
				values = append(values, text)
			}
		}
		return values
	}
	return []string{}
}

func ensureJSONMap(input datatypes.JSONMap) datatypes.JSONMap {
	if input == nil {
		return datatypes.JSONMap{}
	}
	return input
}

func severityForExpiry(expiresAt *time.Time) string {
	if expiresAt == nil {
		return ""
	}
	return severityForDays(int(expiresAt.Sub(time.Now().UTC()).Hours() / 24))
}

func accountCredentialStatus(account model.Account) string {
	if strings.TrimSpace(account.CredentialStatus) != "" {
		return account.CredentialStatus
	}
	if account.Status == "active" {
		return "valid"
	}
	if account.Status == "error" {
		return "invalid"
	}
	return "unknown"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
