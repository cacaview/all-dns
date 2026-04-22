package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"dns-hub/server/internal/model"
	"dns-hub/server/internal/provider"
	"github.com/miekg/dns"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

var defaultResolvers = []string{"1.1.1.1:53", "8.8.8.8:53", "114.114.114.114:53", "223.5.5.5:53", "208.67.222.222:53"}

type PropagationService struct {
	db        *gorm.DB
	resolvers []string
}

type PropagationHistoryItem struct {
	ID             uint              `json:"id"`
	DomainID       uint              `json:"domainId"`
	FQDN           string            `json:"fqdn"`
	Record         datatypes.JSONMap `json:"record"`
	OverallStatus  string            `json:"overallStatus"`
	Summary        string            `json:"summary"`
	MatchedCount   int               `json:"matchedCount"`
	FailedCount    int               `json:"failedCount"`
	PendingCount   int               `json:"pendingCount"`
	TotalResolvers int               `json:"totalResolvers"`
	Results        []map[string]any  `json:"results"`
	CheckedAt      time.Time         `json:"checkedAt"`
	CreatedAt      time.Time         `json:"createdAt"`
}

// WatchOptions controls continuous propagation monitoring behavior.
type WatchOptions struct {
	Resolvers     []string // resolvers to use; defaults to service resolvers if empty
	IntervalSecs  int      // polling interval in seconds; default 30
	MaxAttempts   int      // max polling attempts; default 20
}

func NewPropagationService(db *gorm.DB, resolvers []string) *PropagationService {
	if len(resolvers) == 0 {
		resolvers = defaultResolvers
	}
	return &PropagationService{db: db, resolvers: resolvers}
}

func (s *PropagationService) CheckAndPersist(ctx context.Context, domainID, userID uint, zoneName string, record provider.DNSRecord) (datatypes.JSONMap, error) {
	checkedAt := time.Now().UTC()
	fqdn := buildFQDN(record.Name, zoneName)
	matched := []string{}
	failed := []string{}
	pending := []string{}
	results := []map[string]any{}
	for _, resolver := range s.resolvers {
		status, answers := lookup(ctx, resolver, fqdn, record.Type)
		ok := answerMatches(record, answers)
		reason := propagationReason(status, ok, answers)
		if ok {
			matched = append(matched, resolver)
		} else if status != "ok" {
			failed = append(failed, resolver)
		} else {
			pending = append(pending, resolver)
		}
		results = append(results, map[string]any{
			"resolver": resolver,
			"status":   status,
			"answers":  answers,
			"matched":  ok,
			"reason":   reason,
		})
	}
	overallStatus, summary := summarizePropagation(len(matched), len(failed), len(s.resolvers))
	recordPayload := datatypes.JSONMap{
		"id":       record.ID,
		"type":     record.Type,
		"name":     record.Name,
		"content":  record.Content,
		"ttl":      record.TTL,
		"priority": record.Priority,
		"proxied":  record.Proxied,
		"comment":  record.Comment,
	}
	payload := datatypes.JSONMap{
		"checkedAt":         checkedAt.Format(time.RFC3339),
		"record":            recordPayload,
		"fqdn":              fqdn,
		"matchedResolvers":  matched,
		"failedResolvers":   failed,
		"pendingResolvers":  pending,
		"matchedCount":      len(matched),
		"failedCount":       len(failed),
		"pendingCount":      len(pending),
		"totalResolvers":    len(s.resolvers),
		"isFullyPropagated": len(matched) == len(s.resolvers),
		"overallStatus":     overallStatus,
		"summary":           summary,
		"results":           results,
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("domains").Where("id = ?", domainID).Update("last_propagation_status", payload).Error; err != nil {
			return fmt.Errorf("update domain propagation status: %w", err)
		}
		check := model.PropagationCheck{
			DomainID:          domainID,
			TriggeredByUserID: userID,
			FQDN:              fqdn,
			Record:            recordPayload,
			OverallStatus:     overallStatus,
			Summary:           summary,
			MatchedCount:      len(matched),
			FailedCount:       len(failed),
			PendingCount:      len(pending),
			TotalResolvers:    len(s.resolvers),
			Results:           datatypes.JSONMap{"items": toAnySlice(results)},
			CheckedAt:         checkedAt,
		}
		if err := tx.Create(&check).Error; err != nil {
			return fmt.Errorf("create propagation history: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return payload, nil
}

// Watch continuously polls propagation status until verified, failed, or max attempts reached.
// Returns the final propagation result after the watch completes.
func (s *PropagationService) Watch(ctx context.Context, domainID, userID uint, zoneName string, record provider.DNSRecord, opts WatchOptions) (datatypes.JSONMap, error) {
	resolvers := opts.Resolvers
	if len(resolvers) == 0 {
		resolvers = s.resolvers
	}
	interval := time.Duration(opts.IntervalSecs) * time.Second
	if interval <= 0 {
		interval = 30 * time.Second
	}
	maxAttempts := opts.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 20
	}

	var lastResult datatypes.JSONMap
	for attempt := 0; attempt < maxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return lastResult, ctx.Err()
		case <-time.After(interval):
		}

		// Perform check with the resolved resolver set
		result, err := s.checkWithResolvers(ctx, domainID, userID, zoneName, record, resolvers)
		if err != nil {
			return result, err
		}
		lastResult = result

		status, _ := result["overallStatus"].(string)
		if status == "verified" || status == "failed" {
			return result, nil
		}

		// Continue polling for pending/partial
	}

	// Return last result after max attempts
	return lastResult, nil
}

func (s *PropagationService) checkWithResolvers(ctx context.Context, domainID, userID uint, zoneName string, record provider.DNSRecord, resolvers []string) (datatypes.JSONMap, error) {
	checkedAt := time.Now().UTC()
	fqdn := buildFQDN(record.Name, zoneName)
	matched := []string{}
	failed := []string{}
	pending := []string{}
	results := []map[string]any{}
	for _, resolver := range resolvers {
		status, answers := lookup(ctx, resolver, fqdn, record.Type)
		ok := answerMatches(record, answers)
		reason := propagationReason(status, ok, answers)
		if ok {
			matched = append(matched, resolver)
		} else if status != "ok" {
			failed = append(failed, resolver)
		} else {
			pending = append(pending, resolver)
		}
		results = append(results, map[string]any{
			"resolver": resolver,
			"status":   status,
			"answers":  answers,
			"matched":  ok,
			"reason":   reason,
		})
	}
	overallStatus, summary := summarizePropagation(len(matched), len(failed), len(resolvers))
	recordPayload := datatypes.JSONMap{
		"id":       record.ID,
		"type":     record.Type,
		"name":     record.Name,
		"content":  record.Content,
		"ttl":      record.TTL,
		"priority": record.Priority,
		"proxied":  record.Proxied,
		"comment":  record.Comment,
	}
	payload := datatypes.JSONMap{
		"checkedAt":         checkedAt.Format(time.RFC3339),
		"record":            recordPayload,
		"fqdn":              fqdn,
		"matchedResolvers":  matched,
		"failedResolvers":   failed,
		"pendingResolvers":  pending,
		"matchedCount":      len(matched),
		"failedCount":       len(failed),
		"pendingCount":      len(pending),
		"totalResolvers":    len(resolvers),
		"isFullyPropagated": len(matched) == len(resolvers),
		"overallStatus":     overallStatus,
		"summary":          summary,
		"results":          results,
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("domains").Where("id = ?", domainID).Update("last_propagation_status", payload).Error; err != nil {
			return fmt.Errorf("update domain propagation status: %w", err)
		}
		check := model.PropagationCheck{
			DomainID:          domainID,
			TriggeredByUserID: userID,
			FQDN:              fqdn,
			Record:            recordPayload,
			OverallStatus:     overallStatus,
			Summary:           summary,
			MatchedCount:      len(matched),
			FailedCount:       len(failed),
			PendingCount:      len(pending),
			TotalResolvers:    len(resolvers),
			Results:           datatypes.JSONMap{"items": toAnySlice(results)},
			CheckedAt:         checkedAt,
		}
		if err := tx.Create(&check).Error; err != nil {
			return fmt.Errorf("create propagation history: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return payload, nil
}

func (s *PropagationService) ListHistory(userID, domainID uint) ([]PropagationHistoryItem, error) {
	type row struct {
		model.PropagationCheck
		AccountUserID uint `json:"accountUserId"`
	}
	query := s.db.Table("propagation_checks").
		Select("propagation_checks.*, accounts.user_id as account_user_id").
		Joins("join domains on domains.id = propagation_checks.domain_id").
		Joins("join accounts on accounts.id = domains.account_id").
		Where("accounts.user_id = ?", userID)
	if domainID != 0 {
		query = query.Where("propagation_checks.domain_id = ?", domainID)
	}
	var rows []row
	if err := query.Order("propagation_checks.checked_at desc").Limit(50).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list propagation history: %w", err)
	}
	items := make([]PropagationHistoryItem, 0, len(rows))
	for _, item := range rows {
		items = append(items, PropagationHistoryItem{
			ID:             item.ID,
			DomainID:       item.DomainID,
			FQDN:           item.FQDN,
			Record:         ensureJSONMap(item.Record),
			OverallStatus:  item.OverallStatus,
			Summary:        item.Summary,
			MatchedCount:   item.MatchedCount,
			FailedCount:    item.FailedCount,
			PendingCount:   item.PendingCount,
			TotalResolvers: item.TotalResolvers,
			Results:        propagationHistoryResults(item.Results),
			CheckedAt:      item.CheckedAt,
			CreatedAt:      item.CreatedAt,
		})
	}
	return items, nil
}

func toAnySlice(items []map[string]any) []any {
	result := make([]any, 0, len(items))
	for _, item := range items {
		result = append(result, item)
	}
	return result
}

func propagationHistoryResults(input datatypes.JSONMap) []map[string]any {
	if input == nil {
		return []map[string]any{}
	}
	raw, ok := input["items"]
	if !ok {
		return []map[string]any{}
	}
	items, ok := raw.([]any)
	if !ok {
		return []map[string]any{}
	}
	results := make([]map[string]any, 0, len(items))
	for _, item := range items {
		mapped, ok := item.(map[string]any)
		if !ok {
			continue
		}
		results = append(results, mapped)
	}
	return results
}

func propagationReason(status string, matched bool, answers []string) string {
	if matched {
		return "matched"
	}
	if status != "ok" {
		return "resolver_error"
	}
	if len(answers) == 0 {
		return "no_answer"
	}
	return "value_mismatch"
}

func summarizePropagation(matchedCount, failedCount, totalResolvers int) (string, string) {
	if totalResolvers == 0 {
		return "unknown", "暂无可用解析器"
	}
	if matchedCount == totalResolvers {
		return "verified", fmt.Sprintf("传播检查完成：%d/%d 解析器已返回目标值", matchedCount, totalResolvers)
	}
	if matchedCount > 0 {
		return "partial", fmt.Sprintf("传播进行中：%d/%d 解析器已返回目标值", matchedCount, totalResolvers)
	}
	if failedCount == totalResolvers {
		return "failed", fmt.Sprintf("传播检查失败：%d/%d 解析器请求异常", failedCount, totalResolvers)
	}
	return "pending", fmt.Sprintf("传播未完成：0/%d 解析器返回目标值", totalResolvers)
}

func buildFQDN(name, zone string) string {
	trimmedZone := strings.TrimSuffix(strings.TrimSpace(zone), ".")
	trimmedName := strings.TrimSuffix(strings.TrimSpace(name), ".")
	if trimmedName == "" || trimmedName == "@" {
		return dns.Fqdn(trimmedZone)
	}
	if strings.HasSuffix(trimmedName, trimmedZone) {
		return dns.Fqdn(trimmedName)
	}
	return dns.Fqdn(trimmedName + "." + trimmedZone)
}

func lookup(ctx context.Context, resolver, fqdn, recordType string) (string, []string) {
	message := new(dns.Msg)
	message.SetQuestion(fqdn, dns.StringToType[strings.ToUpper(recordType)])
	client := &dns.Client{Timeout: 5 * time.Second}
	response, _, err := client.ExchangeContext(ctx, message, resolver)
	if err != nil {
		return err.Error(), nil
	}
	answers := make([]string, 0, len(response.Answer))
	for _, answer := range response.Answer {
		answers = append(answers, normalizeAnswer(answer.String()))
	}
	return "ok", answers
}

func answerMatches(record provider.DNSRecord, answers []string) bool {
	expected := strings.Trim(strings.ToLower(record.Content), ".")
	for _, answer := range answers {
		normalized := strings.Trim(strings.ToLower(answer), ".")
		if strings.Contains(normalized, expected) {
			return true
		}
	}
	return false
}

func normalizeAnswer(answer string) string {
	parts := strings.Fields(answer)
	if len(parts) == 0 {
		return answer
	}
	return parts[len(parts)-1]
}
