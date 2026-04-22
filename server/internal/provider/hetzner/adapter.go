package hetzner

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"dns-hub/server/internal/provider"
)

const baseURL = "https://api.hetzner.cloud/v1"

type Adapter struct {
	token  string
	client *http.Client
}

type zonesResponse struct {
	Zones []zone `json:"zones"`
}

type zoneResponse struct {
	Zone zone `json:"zone"`
}

type rrsetsResponse struct {
	RRSets []rrset `json:"rrsets"`
}

type createRRsetResponse struct {
	RRSet  rrset  `json:"rrset"`
	Action action `json:"action"`
}

type actionResponse struct {
	Action action `json:"action"`
}

type zone struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	TTL  *int   `json:"ttl"`
}

type rrset struct {
	ID      string        `json:"id"`
	Name    string        `json:"name"`
	Type    string        `json:"type"`
	TTL     *int          `json:"ttl"`
	Records []rrsetRecord `json:"records"`
}

type rrsetRecord struct {
	Value   string `json:"value"`
	Comment string `json:"comment,omitempty"`
}

type action struct {
	ID           int64  `json:"id"`
	Status       string `json:"status"`
	ErrorMessage string `json:"error_message"`
	Message      string `json:"message"`
}

func init() {
	provider.MustRegister("hetzner", func(config map[string]any) (provider.DNSProvider, error) {
		return New(config)
	})
	provider.MustRegisterDescriptor(provider.Descriptor{
		Key:         "hetzner",
		Label:       "Hetzner",
		Description: "Hetzner Cloud DNS API",
		Fields: []provider.FieldSpec{
			{Key: "api_token", Label: "API Token", Type: provider.FieldTypePassword, Required: true, Placeholder: "Hetzner API Token", HelpText: "使用 Hetzner Console / Cloud API Token"},
		},
		SampleConfig: map[string]any{
			"api_token": "",
		},
	})
}

func New(config map[string]any) (*Adapter, error) {
	token, _ := config["api_token"].(string)
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, fmt.Errorf("hetzner api_token is required")
	}
	return &Adapter{
		token:  token,
		client: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (a *Adapter) Name() string {
	return "hetzner"
}

func (a *Adapter) Validate(ctx context.Context) (*provider.ValidationResult, error) {
	_, err := a.ListDomains(ctx)
	if err != nil {
		return nil, err
	}
	return &provider.ValidationResult{OK: true, Message: "hetzner credentials are valid", CheckedAt: time.Now().UTC()}, nil
}

func (a *Adapter) ListDomains(ctx context.Context) ([]provider.Domain, error) {
	var response zonesResponse
	if err := a.request(ctx, http.MethodGet, "/zones?per_page=500", nil, &response); err != nil {
		return nil, err
	}
	items := make([]provider.Domain, 0, len(response.Zones))
	for _, item := range response.Zones {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		items = append(items, provider.Domain{
			ZoneID:   strconv.FormatInt(item.ID, 10),
			Name:     name,
			Provider: a.Name(),
		})
	}
	return items, nil
}

func (a *Adapter) ListRecords(ctx context.Context, zoneID string) ([]provider.DNSRecord, error) {
	zoneInfo, err := a.getZone(ctx, zoneID)
	if err != nil {
		return nil, err
	}
	rrsets, err := a.listRRsets(ctx, zoneID)
	if err != nil {
		return nil, err
	}
	items := make([]provider.DNSRecord, 0)
	for _, item := range rrsets {
		for _, record := range item.Records {
			items = append(items, mapRecord(zoneInfo, item, record))
		}
	}
	return items, nil
}

func (a *Adapter) UpsertRecord(ctx context.Context, zoneID string, input provider.RecordMutation) (*provider.DNSRecord, error) {
	zoneInfo, err := a.getZone(ctx, zoneID)
	if err != nil {
		return nil, err
	}
	rrsets, err := a.listRRsets(ctx, zoneID)
	if err != nil {
		return nil, err
	}
	trimmedID := strings.TrimSpace(input.ID)
	if trimmedID == "" {
		desired, err := mutationToRecord(input, zoneInfo.Name)
		if err != nil {
			return nil, err
		}
		return a.upsertDesiredRecord(ctx, zoneInfo, rrsets, desired)
	}
	currentName, currentType, currentValue, err := decodeRecordID(trimmedID)
	if err != nil {
		return nil, err
	}
	currentRRset := findRRset(rrsets, currentName, currentType)
	if currentRRset == nil {
		return nil, fmt.Errorf("record %s not found", trimmedID)
	}
	currentRecord, ok := findProviderRecord(zoneInfo, *currentRRset, currentValue)
	if !ok {
		return nil, fmt.Errorf("record %s not found", trimmedID)
	}
	desired, err := mergeRecord(currentRecord, input, zoneInfo.Name)
	if err != nil {
		return nil, err
	}
	if sameRRsetIdentity(currentName, currentType, desired.Name, desired.Type, zoneInfo.Name) {
		if err := a.replaceRecordInRRset(ctx, zoneInfo, *currentRRset, currentValue, desired); err != nil {
			return nil, err
		}
		return a.readBackRecord(ctx, zoneInfo, desired)
	}
	if err := a.removeRecordFromRRset(ctx, zoneInfo, *currentRRset, currentValue); err != nil {
		return nil, err
	}
	updatedRRsets, err := a.listRRsets(ctx, zoneID)
	if err != nil {
		return nil, err
	}
	return a.upsertDesiredRecord(ctx, zoneInfo, updatedRRsets, desired)
}

func (a *Adapter) DeleteRecord(ctx context.Context, zoneID string, recordID string) error {
	zoneInfo, err := a.getZone(ctx, zoneID)
	if err != nil {
		return err
	}
	name, recordType, value, err := decodeRecordID(recordID)
	if err != nil {
		return err
	}
	rrsets, err := a.listRRsets(ctx, zoneID)
	if err != nil {
		return err
	}
	currentRRset := findRRset(rrsets, name, recordType)
	if currentRRset == nil {
		return fmt.Errorf("record %s not found", strings.TrimSpace(recordID))
	}
	return a.removeRecordFromRRset(ctx, zoneInfo, *currentRRset, value)
}

func (a *Adapter) ExportConfig() map[string]any {
	return map[string]any{
		"api_token": a.token,
	}
}

func (a *Adapter) getZone(ctx context.Context, zoneID string) (zone, error) {
	trimmed := strings.TrimSpace(zoneID)
	if trimmed == "" {
		return zone{}, fmt.Errorf("zone id is required")
	}
	var response zoneResponse
	if err := a.request(ctx, http.MethodGet, "/zones/"+url.PathEscape(trimmed), nil, &response); err != nil {
		return zone{}, err
	}
	if response.Zone.ID == 0 {
		return zone{}, fmt.Errorf("zone %s not found", trimmed)
	}
	return response.Zone, nil
}

func (a *Adapter) listRRsets(ctx context.Context, zoneID string) ([]rrset, error) {
	trimmed := strings.TrimSpace(zoneID)
	if trimmed == "" {
		return nil, fmt.Errorf("zone id is required")
	}
	var response rrsetsResponse
	path := fmt.Sprintf("/zones/%s/rrsets?per_page=500", url.PathEscape(trimmed))
	if err := a.request(ctx, http.MethodGet, path, nil, &response); err != nil {
		return nil, err
	}
	return response.RRSets, nil
}

func (a *Adapter) upsertDesiredRecord(ctx context.Context, zoneInfo zone, rrsets []rrset, desired provider.DNSRecord) (*provider.DNSRecord, error) {
	name := normalizeRecordName(desired.Name, zoneInfo.Name)
	recordType := strings.ToUpper(strings.TrimSpace(desired.Type))
	value, err := buildRecordValue(recordType, desired.Content, desired.Priority)
	if err != nil {
		return nil, err
	}
	comment := strings.TrimSpace(desired.Comment)
	currentRRset := findRRset(rrsets, name, recordType)
	if currentRRset == nil {
		payload := map[string]any{
			"name":    name,
			"type":    recordType,
			"records": []rrsetRecord{{Value: value, Comment: comment}},
		}
		if desired.TTL > 0 {
			payload["ttl"] = desired.TTL
		}
		var response createRRsetResponse
		if err := a.request(ctx, http.MethodPost, fmt.Sprintf("/zones/%s/rrsets", url.PathEscape(strconv.FormatInt(zoneInfo.ID, 10))), payload, &response); err != nil {
			return nil, err
		}
		if err := a.waitAction(ctx, zoneInfo.ID, response.Action.ID); err != nil {
			return nil, err
		}
		return a.readBackRecord(ctx, zoneInfo, desired)
	}
	records := cloneRRsetRecords(currentRRset.Records)
	replaced := false
	for index := range records {
		if strings.TrimSpace(records[index].Value) == value {
			records[index].Comment = comment
			replaced = true
			break
		}
	}
	if !replaced {
		records = append(records, rrsetRecord{Value: value, Comment: comment})
	}
	if err := a.setRRsetRecords(ctx, zoneInfo.ID, currentRRset.Name, currentRRset.Type, records); err != nil {
		return nil, err
	}
	if desired.TTL > 0 && desired.TTL != effectiveTTL(zoneInfo, *currentRRset) {
		if err := a.changeRRsetTTL(ctx, zoneInfo.ID, currentRRset.Name, currentRRset.Type, desired.TTL); err != nil {
			return nil, err
		}
	}
	return a.readBackRecord(ctx, zoneInfo, desired)
}

func (a *Adapter) replaceRecordInRRset(ctx context.Context, zoneInfo zone, current rrset, oldValue string, desired provider.DNSRecord) error {
	desiredValue, err := buildRecordValue(desired.Type, desired.Content, desired.Priority)
	if err != nil {
		return err
	}
	records := cloneRRsetRecords(current.Records)
	replaced := false
	for index := range records {
		if strings.TrimSpace(records[index].Value) != strings.TrimSpace(oldValue) {
			continue
		}
		records[index].Value = desiredValue
		records[index].Comment = strings.TrimSpace(desired.Comment)
		replaced = true
		break
	}
	if !replaced {
		return fmt.Errorf("record %s not found", encodeRecordID(current.Name, current.Type, oldValue))
	}
	if err := a.setRRsetRecords(ctx, zoneInfo.ID, current.Name, current.Type, records); err != nil {
		return err
	}
	if desired.TTL > 0 && desired.TTL != effectiveTTL(zoneInfo, current) {
		return a.changeRRsetTTL(ctx, zoneInfo.ID, current.Name, current.Type, desired.TTL)
	}
	return nil
}

func (a *Adapter) removeRecordFromRRset(ctx context.Context, zoneInfo zone, current rrset, value string) error {
	if len(current.Records) <= 1 {
		return a.deleteRRset(ctx, zoneInfo.ID, current.Name, current.Type)
	}
	filtered := make([]rrsetRecord, 0, len(current.Records)-1)
	removed := false
	for _, item := range current.Records {
		if strings.TrimSpace(item.Value) == strings.TrimSpace(value) {
			removed = true
			continue
		}
		filtered = append(filtered, item)
	}
	if !removed {
		return fmt.Errorf("record %s not found", encodeRecordID(current.Name, current.Type, value))
	}
	return a.setRRsetRecords(ctx, zoneInfo.ID, current.Name, current.Type, filtered)
}

func (a *Adapter) setRRsetRecords(ctx context.Context, zoneID int64, name, recordType string, records []rrsetRecord) error {
	payload := map[string]any{"records": records}
	var response actionResponse
	path := fmt.Sprintf("/zones/%s/rrsets/%s/%s/actions/set_records", url.PathEscape(strconv.FormatInt(zoneID, 10)), pathRecordName(name), pathRecordType(recordType))
	if err := a.request(ctx, http.MethodPost, path, payload, &response); err != nil {
		return err
	}
	return a.waitAction(ctx, zoneID, response.Action.ID)
}

func (a *Adapter) changeRRsetTTL(ctx context.Context, zoneID int64, name, recordType string, ttl int) error {
	var response actionResponse
	path := fmt.Sprintf("/zones/%s/rrsets/%s/%s/actions/change_ttl", url.PathEscape(strconv.FormatInt(zoneID, 10)), pathRecordName(name), pathRecordType(recordType))
	if err := a.request(ctx, http.MethodPost, path, map[string]any{"ttl": ttl}, &response); err != nil {
		return err
	}
	return a.waitAction(ctx, zoneID, response.Action.ID)
}

func (a *Adapter) deleteRRset(ctx context.Context, zoneID int64, name, recordType string) error {
	var response actionResponse
	path := fmt.Sprintf("/zones/%s/rrsets/%s/%s", url.PathEscape(strconv.FormatInt(zoneID, 10)), pathRecordName(name), pathRecordType(recordType))
	if err := a.request(ctx, http.MethodDelete, path, nil, &response); err != nil {
		return err
	}
	return a.waitAction(ctx, zoneID, response.Action.ID)
}

func (a *Adapter) waitAction(ctx context.Context, zoneID int64, actionID int64) error {
	if actionID == 0 {
		return nil
	}
	for attempt := 0; attempt < 20; attempt++ {
		var response actionResponse
		path := fmt.Sprintf("/zones/%s/actions/%d", url.PathEscape(strconv.FormatInt(zoneID, 10)), actionID)
		if err := a.request(ctx, http.MethodGet, path, nil, &response); err != nil {
			return err
		}
		switch strings.ToLower(strings.TrimSpace(response.Action.Status)) {
		case "success", "completed", "done":
			return nil
		case "error", "failed":
			message := firstNonEmpty(strings.TrimSpace(response.Action.ErrorMessage), strings.TrimSpace(response.Action.Message), "hetzner action failed")
			return fmt.Errorf("%s", message)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1500 * time.Millisecond):
		}
	}
	return fmt.Errorf("timed out waiting for hetzner action %d", actionID)
}

func (a *Adapter) readBackRecord(ctx context.Context, zoneInfo zone, desired provider.DNSRecord) (*provider.DNSRecord, error) {
	records, err := a.ListRecords(ctx, strconv.FormatInt(zoneInfo.ID, 10))
	if err != nil {
		return nil, err
	}
	desiredValue, err := buildRecordValue(desired.Type, desired.Content, desired.Priority)
	if err != nil {
		return nil, err
	}
	for index := range records {
		item := records[index]
		itemValue, buildErr := buildRecordValue(item.Type, item.Content, item.Priority)
		if buildErr != nil {
			continue
		}
		if normalizeRecordName(item.Name, zoneInfo.Name) == normalizeRecordName(desired.Name, zoneInfo.Name) && strings.EqualFold(strings.TrimSpace(item.Type), strings.TrimSpace(desired.Type)) && strings.TrimSpace(itemValue) == strings.TrimSpace(desiredValue) {
			return &item, nil
		}
	}
	fallback := desired
	fallback.ID = encodeRecordID(normalizeRecordName(desired.Name, zoneInfo.Name), desired.Type, desiredValue)
	if fallback.TTL <= 0 {
		fallback.TTL = defaultZoneTTL(zoneInfo)
	}
	return &fallback, nil
}

func (a *Adapter) request(ctx context.Context, method, path string, body any, target any) error {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal hetzner request: %w", err)
		}
		reader = bytes.NewBuffer(payload)
	}
	request, err := http.NewRequestWithContext(ctx, method, baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("build hetzner request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+a.token)
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := a.client.Do(request)
	if err != nil {
		return fmt.Errorf("call hetzner api: %w", err)
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("read hetzner response: %w", err)
	}
	if response.StatusCode >= 400 {
		return fmt.Errorf("hetzner api returned %s: %s", response.Status, strings.TrimSpace(string(payload)))
	}
	if target == nil || len(payload) == 0 {
		return nil
	}
	if err := json.Unmarshal(payload, target); err != nil {
		return fmt.Errorf("decode hetzner response: %w", err)
	}
	return nil
}

func mapRecord(zoneInfo zone, current rrset, record rrsetRecord) provider.DNSRecord {
	content, priority := splitRecordValue(current.Type, record.Value)
	return provider.DNSRecord{
		ID:       encodeRecordID(current.Name, current.Type, record.Value),
		Type:     strings.ToUpper(strings.TrimSpace(current.Type)),
		Name:     normalizeRecordName(current.Name, zoneInfo.Name),
		Content:  content,
		TTL:      effectiveTTL(zoneInfo, current),
		Priority: priority,
		Comment:  strings.TrimSpace(record.Comment),
	}
}

func mutationToRecord(input provider.RecordMutation, domainName string) (provider.DNSRecord, error) {
	recordType := strings.ToUpper(strings.TrimSpace(input.Type))
	if recordType == "" {
		return provider.DNSRecord{}, fmt.Errorf("record type is required")
	}
	content := strings.TrimSpace(input.Content)
	if content == "" {
		return provider.DNSRecord{}, fmt.Errorf("record content is required")
	}
	ttl := input.TTL
	if ttl <= 0 {
		ttl = 60
	}
	return provider.DNSRecord{
		Type:     recordType,
		Name:     normalizeRecordName(strings.TrimSpace(input.Name), domainName),
		Content:  content,
		TTL:      ttl,
		Priority: input.Priority,
		Comment:  input.Comment,
	}, nil
}

func mergeRecord(current provider.DNSRecord, input provider.RecordMutation, domainName string) (provider.DNSRecord, error) {
	updated := current
	if strings.TrimSpace(input.Type) != "" {
		updated.Type = strings.ToUpper(strings.TrimSpace(input.Type))
	}
	if input.Name != "" {
		updated.Name = normalizeRecordName(strings.TrimSpace(input.Name), domainName)
	}
	if strings.TrimSpace(input.Content) != "" {
		updated.Content = strings.TrimSpace(input.Content)
	}
	if input.TTL > 0 {
		updated.TTL = input.TTL
	}
	if input.Priority != nil {
		updated.Priority = input.Priority
	}
	if input.Comment != "" {
		updated.Comment = input.Comment
	}
	if strings.TrimSpace(updated.Type) == "" {
		return provider.DNSRecord{}, fmt.Errorf("record type is required")
	}
	if strings.TrimSpace(updated.Content) == "" {
		return provider.DNSRecord{}, fmt.Errorf("record content is required")
	}
	return updated, nil
}

func findRRset(rrsets []rrset, name, recordType string) *rrset {
	normalizedName := normalizeComparableName(name)
	normalizedType := strings.ToUpper(strings.TrimSpace(recordType))
	for index := range rrsets {
		if normalizeComparableName(rrsets[index].Name) == normalizedName && strings.ToUpper(strings.TrimSpace(rrsets[index].Type)) == normalizedType {
			return &rrsets[index]
		}
	}
	return nil
}

func findProviderRecord(zoneInfo zone, current rrset, value string) (provider.DNSRecord, bool) {
	for _, item := range current.Records {
		if strings.TrimSpace(item.Value) == strings.TrimSpace(value) {
			return mapRecord(zoneInfo, current, item), true
		}
	}
	return provider.DNSRecord{}, false
}

func sameRRsetIdentity(currentName, currentType, desiredName, desiredType, domainName string) bool {
	return normalizeComparableName(currentName) == normalizeComparableName(normalizeRecordName(desiredName, domainName)) && strings.EqualFold(strings.TrimSpace(currentType), strings.TrimSpace(desiredType))
}

func buildRecordValue(recordType, content string, priority *int) (string, error) {
	trimmedType := strings.ToUpper(strings.TrimSpace(recordType))
	trimmedContent := strings.TrimSpace(content)
	if trimmedType == "" {
		return "", fmt.Errorf("record type is required")
	}
	if trimmedContent == "" {
		return "", fmt.Errorf("record content is required")
	}
	if trimmedType == "MX" && priority != nil {
		return fmt.Sprintf("%d %s", *priority, trimmedContent), nil
	}
	return trimmedContent, nil
}

func splitRecordValue(recordType, value string) (string, *int) {
	trimmedType := strings.ToUpper(strings.TrimSpace(recordType))
	trimmedValue := strings.TrimSpace(value)
	if trimmedType != "MX" {
		return trimmedValue, nil
	}
	parts := strings.Fields(trimmedValue)
	if len(parts) < 2 {
		return trimmedValue, nil
	}
	priority, err := strconv.Atoi(parts[0])
	if err != nil {
		return trimmedValue, nil
	}
	content := strings.Join(parts[1:], " ")
	return content, &priority
}

func encodeRecordID(name, recordType, value string) string {
	return normalizeComparableName(name) + "|" + strings.ToUpper(strings.TrimSpace(recordType)) + "|" + base64.RawURLEncoding.EncodeToString([]byte(strings.TrimSpace(value)))
}

func decodeRecordID(recordID string) (string, string, string, error) {
	parts := strings.Split(strings.TrimSpace(recordID), "|")
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("invalid hetzner record id")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return "", "", "", fmt.Errorf("invalid hetzner record id")
	}
	return normalizeComparableName(parts[0]), strings.ToUpper(strings.TrimSpace(parts[1])), strings.TrimSpace(string(decoded)), nil
}

func cloneRRsetRecords(input []rrsetRecord) []rrsetRecord {
	items := make([]rrsetRecord, len(input))
	copy(items, input)
	return items
}

func effectiveTTL(zoneInfo zone, current rrset) int {
	if current.TTL != nil && *current.TTL > 0 {
		return *current.TTL
	}
	return defaultZoneTTL(zoneInfo)
}

func defaultZoneTTL(zoneInfo zone) int {
	if zoneInfo.TTL != nil && *zoneInfo.TTL > 0 {
		return *zoneInfo.TTL
	}
	return 60
}

func pathRecordName(name string) string {
	return url.PathEscape(normalizeComparableName(name))
}

func pathRecordType(recordType string) string {
	return url.PathEscape(strings.ToUpper(strings.TrimSpace(recordType)))
}

func normalizeRecordName(name, domainName string) string {
	trimmedName := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(name)), ".")
	if trimmedName == "" || trimmedName == "@" {
		return "@"
	}
	trimmedDomain := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(domainName)), ".")
	if trimmedName == trimmedDomain {
		return "@"
	}
	suffix := "." + trimmedDomain
	if trimmedDomain != "" && strings.HasSuffix(trimmedName, suffix) {
		short := strings.TrimSuffix(trimmedName, suffix)
		short = strings.TrimSuffix(short, ".")
		if short == "" {
			return "@"
		}
		return short
	}
	return trimmedName
}

func normalizeComparableName(name string) string {
	trimmed := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(name)), ".")
	if trimmed == "" {
		return "@"
	}
	return trimmed
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
