package huawei

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"dns-hub/server/internal/provider"
)

const (
	defaultRegion      = "ap-southeast-3"
	defaultEndpoint    = "https://dns.ap-southeast-3.myhuaweicloud.com"
	defaultIAMEndpoint = "https://iam.myhuaweicloud.com"
	defaultTTL         = 300
	pageLimit          = 500
)

type Adapter struct {
	username    string
	password    string
	domainName  string
	region      string
	endpoint    string
	iamEndpoint string
	client      *http.Client

	tokenMu     sync.Mutex
	token       string
	tokenExpiry time.Time
}

type zonesResponse struct {
	Zones    []zone `json:"zones"`
	Metadata struct {
		TotalCount int `json:"total_count"`
	} `json:"metadata"`
}

type zone struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	TTL      int    `json:"ttl"`
	ZoneType string `json:"zone_type"`
}

type recordsetsResponse struct {
	Recordsets []recordset `json:"recordsets"`
	Metadata   struct {
		TotalCount int `json:"total_count"`
	} `json:"metadata"`
}

type recordset struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	TTL     int      `json:"ttl"`
	Records []string `json:"records"`
}

type iamTokenResponse struct {
	Token struct {
		ExpiresAt string `json:"expires_at"`
	} `json:"token"`
}

func init() {
	provider.MustRegister("huawei", func(config map[string]any) (provider.DNSProvider, error) {
		return New(config)
	})
	provider.MustRegisterDescriptor(provider.Descriptor{
		Key:         "huawei",
		Label:       "华为云 DNS",
		Description: "Huawei Cloud DNS API",
		Fields: []provider.FieldSpec{
			{Key: "username", Label: "IAM Username", Type: provider.FieldTypeText, Required: true, Placeholder: "用户名", HelpText: "华为云 IAM 用户名"},
			{Key: "password", Label: "IAM Password", Type: provider.FieldTypePassword, Required: true, Placeholder: "密码", HelpText: "华为云 IAM 用户密码"},
			{Key: "domain_name", Label: "Account Name", Type: provider.FieldTypeText, Required: true, Placeholder: "账号名", HelpText: "华为云账号名 / IAM Domain Name"},
			{Key: "region", Label: "Region", Type: provider.FieldTypeText, Required: false, Placeholder: defaultRegion, HelpText: "公共 DNS 默认使用 ap-southeast-3"},
			{Key: "endpoint", Label: "DNS Endpoint", Type: provider.FieldTypeText, Required: false, Placeholder: defaultEndpoint, HelpText: "可选，默认使用华为云 DNS API Endpoint"},
			{Key: "iam_endpoint", Label: "IAM Endpoint", Type: provider.FieldTypeText, Required: false, Placeholder: defaultIAMEndpoint, HelpText: "可选，默认使用华为云 IAM Endpoint"},
		},
		SampleConfig: map[string]any{
			"username":     "",
			"password":     "",
			"domain_name":  "",
			"region":       defaultRegion,
			"endpoint":     defaultEndpoint,
			"iam_endpoint": defaultIAMEndpoint,
		},
	})
}

func New(config map[string]any) (*Adapter, error) {
	username, _ := config["username"].(string)
	password, _ := config["password"].(string)
	domainName, _ := config["domain_name"].(string)
	region, _ := config["region"].(string)
	endpoint, _ := config["endpoint"].(string)
	iamEndpoint, _ := config["iam_endpoint"].(string)

	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)
	domainName = strings.TrimSpace(domainName)
	region = strings.TrimSpace(region)
	endpoint = strings.TrimSpace(endpoint)
	iamEndpoint = strings.TrimSpace(iamEndpoint)

	if username == "" {
		return nil, fmt.Errorf("huawei username is required")
	}
	if password == "" {
		return nil, fmt.Errorf("huawei password is required")
	}
	if domainName == "" {
		return nil, fmt.Errorf("huawei domain_name is required")
	}
	if region == "" {
		region = defaultRegion
	}
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	if iamEndpoint == "" {
		iamEndpoint = defaultIAMEndpoint
	}
	endpoint = normalizeEndpoint(endpoint)
	iamEndpoint = normalizeEndpoint(iamEndpoint)
	if _, err := url.Parse(endpoint); err != nil {
		return nil, fmt.Errorf("parse huawei endpoint: %w", err)
	}
	if _, err := url.Parse(iamEndpoint); err != nil {
		return nil, fmt.Errorf("parse huawei iam_endpoint: %w", err)
	}

	return &Adapter{
		username:    username,
		password:    password,
		domainName:  domainName,
		region:      region,
		endpoint:    strings.TrimRight(endpoint, "/"),
		iamEndpoint: strings.TrimRight(iamEndpoint, "/"),
		client:      &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (a *Adapter) Name() string {
	return "huawei"
}

func (a *Adapter) Validate(ctx context.Context) (*provider.ValidationResult, error) {
	_, err := a.ListDomains(ctx)
	if err != nil {
		return nil, err
	}
	return &provider.ValidationResult{OK: true, Message: "huawei credentials are valid", CheckedAt: time.Now().UTC()}, nil
}

func (a *Adapter) ListDomains(ctx context.Context) ([]provider.Domain, error) {
	items := make([]provider.Domain, 0)
	marker := ""
	for {
		query := url.Values{}
		query.Set("type", "public")
		query.Set("limit", fmt.Sprintf("%d", pageLimit))
		if marker != "" {
			query.Set("marker", marker)
		}
		var response zonesResponse
		if err := a.request(ctx, http.MethodGet, "/v2/zones", query, nil, &response); err != nil {
			return nil, err
		}
		for _, item := range response.Zones {
			name := strings.TrimSpace(strings.TrimSuffix(item.Name, "."))
			if name == "" {
				continue
			}
			items = append(items, provider.Domain{ZoneID: strings.TrimSpace(item.ID), Name: name, Provider: a.Name()})
		}
		if len(response.Zones) == 0 || len(response.Zones) < pageLimit {
			break
		}
		marker = strings.TrimSpace(response.Zones[len(response.Zones)-1].ID)
		if marker == "" {
			break
		}
	}
	return items, nil
}

func (a *Adapter) ListRecords(ctx context.Context, zoneID string) ([]provider.DNSRecord, error) {
	zoneInfo, err := a.getZone(ctx, zoneID)
	if err != nil {
		return nil, err
	}
	recordsets, err := a.listRecordsets(ctx, zoneID)
	if err != nil {
		return nil, err
	}
	items := make([]provider.DNSRecord, 0)
	for _, item := range recordsets {
		for _, value := range item.Records {
			items = append(items, mapRecord(zoneInfo, item, value))
		}
	}
	return items, nil
}

func (a *Adapter) UpsertRecord(ctx context.Context, zoneID string, input provider.RecordMutation) (*provider.DNSRecord, error) {
	zoneInfo, err := a.getZone(ctx, zoneID)
	if err != nil {
		return nil, err
	}
	recordsets, err := a.listRecordsets(ctx, zoneID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(input.ID) == "" {
		desired, err := mutationToRecord(input, zoneInfo.Name)
		if err != nil {
			return nil, err
		}
		return a.upsertDesiredRecord(ctx, zoneInfo, recordsets, desired)
	}

	recordsetID, rawValue, err := decodeRecordID(input.ID)
	if err != nil {
		return nil, err
	}
	current := findRecordsetByID(recordsets, recordsetID)
	if current == nil {
		return nil, fmt.Errorf("record %s not found", strings.TrimSpace(input.ID))
	}
	currentRecord := findRecordInRecordset(zoneInfo, *current, rawValue)
	if currentRecord == nil {
		return nil, fmt.Errorf("record %s not found", strings.TrimSpace(input.ID))
	}
	desired, err := mergeRecord(*currentRecord, input, zoneInfo.Name)
	if err != nil {
		return nil, err
	}
	if sameRecordsetIdentity(current.Name, current.Type, desired.Name, desired.Type, zoneInfo.Name) {
		if err := a.replaceRecordInRecordset(ctx, zoneInfo.ID, *current, rawValue, desired); err != nil {
			return nil, err
		}
		return a.readBackRecord(ctx, zoneInfo, desired)
	}
	if err := a.removeRecordFromRecordset(ctx, zoneInfo.ID, *current, rawValue); err != nil {
		return nil, err
	}
	updated, err := a.listRecordsets(ctx, zoneInfo.ID)
	if err != nil {
		return nil, err
	}
	return a.upsertDesiredRecord(ctx, zoneInfo, updated, desired)
}

func (a *Adapter) DeleteRecord(ctx context.Context, zoneID string, recordID string) error {
	recordsetID, rawValue, err := decodeRecordID(recordID)
	if err != nil {
		return err
	}
	recordsets, err := a.listRecordsets(ctx, zoneID)
	if err != nil {
		return err
	}
	current := findRecordsetByID(recordsets, recordsetID)
	if current == nil {
		return fmt.Errorf("record %s not found", strings.TrimSpace(recordID))
	}
	return a.removeRecordFromRecordset(ctx, zoneID, *current, rawValue)
}

func (a *Adapter) ExportConfig() map[string]any {
	return map[string]any{
		"username":     a.username,
		"password":     a.password,
		"domain_name":  a.domainName,
		"region":       a.region,
		"endpoint":     a.endpoint,
		"iam_endpoint": a.iamEndpoint,
	}
}

func (a *Adapter) getZone(ctx context.Context, zoneID string) (zone, error) {
	trimmed := strings.TrimSpace(zoneID)
	if trimmed == "" {
		return zone{}, fmt.Errorf("zone id is required")
	}
	var response zone
	if err := a.request(ctx, http.MethodGet, "/v2/zones/"+url.PathEscape(trimmed), nil, nil, &response); err != nil {
		return zone{}, err
	}
	if strings.TrimSpace(response.ID) == "" {
		return zone{}, fmt.Errorf("zone %s not found", trimmed)
	}
	return response, nil
}

func (a *Adapter) listRecordsets(ctx context.Context, zoneID string) ([]recordset, error) {
	trimmed := strings.TrimSpace(zoneID)
	if trimmed == "" {
		return nil, fmt.Errorf("zone id is required")
	}
	items := make([]recordset, 0)
	marker := ""
	for {
		query := url.Values{}
		query.Set("limit", fmt.Sprintf("%d", pageLimit))
		if marker != "" {
			query.Set("marker", marker)
		}
		var response recordsetsResponse
		path := "/v2/zones/" + url.PathEscape(trimmed) + "/recordsets"
		if err := a.request(ctx, http.MethodGet, path, query, nil, &response); err != nil {
			return nil, err
		}
		items = append(items, response.Recordsets...)
		if len(response.Recordsets) == 0 || len(response.Recordsets) < pageLimit {
			break
		}
		marker = strings.TrimSpace(response.Recordsets[len(response.Recordsets)-1].ID)
		if marker == "" {
			break
		}
	}
	return items, nil
}

func (a *Adapter) upsertDesiredRecord(ctx context.Context, zoneInfo zone, recordsets []recordset, desired provider.DNSRecord) (*provider.DNSRecord, error) {
	recordsetName := fqdnRecordName(desired.Name, zoneInfo.Name)
	recordType := strings.ToUpper(strings.TrimSpace(desired.Type))
	rawValue, err := buildRecordValue(recordType, desired.Content, desired.Priority)
	if err != nil {
		return nil, err
	}
	current := findRecordset(recordsets, recordsetName, recordType)
	if current == nil {
		payload := map[string]any{
			"name":    recordsetName,
			"type":    recordType,
			"ttl":     effectiveTTLInt(desired.TTL),
			"records": []string{rawValue},
		}
		path := "/v2/zones/" + url.PathEscape(zoneInfo.ID) + "/recordsets"
		if err := a.request(ctx, http.MethodPost, path, nil, payload, nil); err != nil {
			return nil, err
		}
		return a.readBackRecord(ctx, zoneInfo, desired)
	}
	records := cloneStrings(current.Records)
	found := false
	for _, item := range records {
		if strings.TrimSpace(item) == strings.TrimSpace(rawValue) {
			found = true
			break
		}
	}
	if !found {
		records = append(records, rawValue)
	}
	if err := a.updateRecordset(ctx, zoneInfo.ID, current.ID, recordsetName, recordType, effectiveTTLFromRecord(*current, desired.TTL), records); err != nil {
		return nil, err
	}
	return a.readBackRecord(ctx, zoneInfo, desired)
}

func (a *Adapter) replaceRecordInRecordset(ctx context.Context, zoneID string, current recordset, oldValue string, desired provider.DNSRecord) error {
	newValue, err := buildRecordValue(desired.Type, desired.Content, desired.Priority)
	if err != nil {
		return err
	}
	records := cloneStrings(current.Records)
	replaced := false
	for index := range records {
		if strings.TrimSpace(records[index]) != strings.TrimSpace(oldValue) {
			continue
		}
		records[index] = newValue
		replaced = true
		break
	}
	if !replaced {
		return fmt.Errorf("record %s not found", encodeRecordID(current.ID, oldValue))
	}
	return a.updateRecordset(ctx, zoneID, current.ID, fqdnRecordName(desired.Name, ""), strings.ToUpper(strings.TrimSpace(desired.Type)), effectiveTTLFromRecord(current, desired.TTL), records)
}

func (a *Adapter) removeRecordFromRecordset(ctx context.Context, zoneID string, current recordset, rawValue string) error {
	filtered := make([]string, 0, len(current.Records))
	removed := false
	for _, item := range current.Records {
		if strings.TrimSpace(item) == strings.TrimSpace(rawValue) {
			removed = true
			continue
		}
		filtered = append(filtered, item)
	}
	if !removed {
		return fmt.Errorf("record %s not found", encodeRecordID(current.ID, rawValue))
	}
	if len(filtered) == 0 {
		return a.request(ctx, http.MethodDelete, "/v2/zones/"+url.PathEscape(zoneID)+"/recordsets/"+url.PathEscape(current.ID), nil, nil, nil)
	}
	return a.updateRecordset(ctx, zoneID, current.ID, current.Name, current.Type, effectiveTTL(current), filtered)
}

func (a *Adapter) updateRecordset(ctx context.Context, zoneID, recordsetID, name, recordType string, ttl int, records []string) error {
	payload := map[string]any{
		"name":    fqdnRecordName(name, ""),
		"type":    strings.ToUpper(strings.TrimSpace(recordType)),
		"ttl":     effectiveTTLInt(ttl),
		"records": records,
	}
	path := "/v2/zones/" + url.PathEscape(strings.TrimSpace(zoneID)) + "/recordsets/" + url.PathEscape(strings.TrimSpace(recordsetID))
	return a.request(ctx, http.MethodPut, path, nil, payload, nil)
}

func (a *Adapter) readBackRecord(ctx context.Context, zoneInfo zone, desired provider.DNSRecord) (*provider.DNSRecord, error) {
	recordsets, err := a.listRecordsets(ctx, zoneInfo.ID)
	if err != nil {
		return nil, err
	}
	desiredValue, err := buildRecordValue(desired.Type, desired.Content, desired.Priority)
	if err != nil {
		return nil, err
	}
	targetName := normalizeComparableName(fqdnRecordName(desired.Name, zoneInfo.Name))
	targetType := strings.ToUpper(strings.TrimSpace(desired.Type))
	for _, item := range recordsets {
		if normalizeComparableName(item.Name) != targetName || strings.ToUpper(strings.TrimSpace(item.Type)) != targetType {
			continue
		}
		for _, value := range item.Records {
			if strings.TrimSpace(value) != strings.TrimSpace(desiredValue) {
				continue
			}
			record := mapRecord(zoneInfo, item, value)
			return &record, nil
		}
	}
	return nil, fmt.Errorf("record not found after huawei change")
}

func (a *Adapter) request(ctx context.Context, method, path string, query url.Values, body any, target any) error {
	token, err := a.getToken(ctx)
	if err != nil {
		return err
	}
	requestURL := a.endpoint + path
	if len(query) > 0 {
		requestURL += "?" + query.Encode()
	}
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal huawei request: %w", err)
		}
		reader = bytes.NewBuffer(payload)
	}
	request, err := http.NewRequestWithContext(ctx, method, requestURL, reader)
	if err != nil {
		return fmt.Errorf("build huawei request: %w", err)
	}
	request.Header.Set("X-Auth-Token", token)
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := a.client.Do(request)
	if err != nil {
		return fmt.Errorf("call huawei api: %w", err)
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("read huawei response: %w", err)
	}
	if response.StatusCode >= 400 {
		return fmt.Errorf("huawei api returned %s: %s", response.Status, strings.TrimSpace(string(payload)))
	}
	if target == nil || len(payload) == 0 {
		return nil
	}
	if err := json.Unmarshal(payload, target); err != nil {
		return fmt.Errorf("decode huawei response: %w", err)
	}
	return nil
}

func (a *Adapter) getToken(ctx context.Context) (string, error) {
	a.tokenMu.Lock()
	defer a.tokenMu.Unlock()
	if strings.TrimSpace(a.token) != "" && time.Now().UTC().Before(a.tokenExpiry) {
		return a.token, nil
	}
	payload := map[string]any{
		"auth": map[string]any{
			"identity": map[string]any{
				"methods": []string{"password"},
				"password": map[string]any{
					"user": map[string]any{
						"name":     a.username,
						"password": a.password,
						"domain": map[string]any{
							"name": a.domainName,
						},
					},
				},
			},
			"scope": map[string]any{
				"project": map[string]any{
					"name": a.region,
				},
			},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal huawei iam request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, a.iamEndpoint+"/v3/auth/tokens", bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("build huawei iam request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	response, err := a.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("call huawei iam api: %w", err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return "", fmt.Errorf("read huawei iam response: %w", err)
	}
	if response.StatusCode >= 400 {
		return "", fmt.Errorf("huawei iam api returned %s: %s", response.Status, strings.TrimSpace(string(responseBody)))
	}
	token := strings.TrimSpace(response.Header.Get("X-Subject-Token"))
	if token == "" {
		return "", fmt.Errorf("huawei iam token missing in response")
	}
	expiry := time.Now().UTC().Add(50 * time.Minute)
	var tokenResponse iamTokenResponse
	if err := json.Unmarshal(responseBody, &tokenResponse); err == nil {
		if parsed, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(tokenResponse.Token.ExpiresAt)); parseErr == nil {
			expiry = parsed.Add(-2 * time.Minute)
		}
	}
	a.token = token
	a.tokenExpiry = expiry
	return a.token, nil
}

func mapRecord(zoneInfo zone, current recordset, rawValue string) provider.DNSRecord {
	content, priority := splitRecordValue(current.Type, rawValue)
	return provider.DNSRecord{
		ID:       encodeRecordID(current.ID, rawValue),
		Type:     strings.ToUpper(strings.TrimSpace(current.Type)),
		Name:     normalizeRecordName(current.Name, zoneInfo.Name),
		Content:  content,
		TTL:      effectiveTTL(current),
		Priority: priority,
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
	return provider.DNSRecord{
		Type:     recordType,
		Name:     normalizeRecordName(strings.TrimSpace(input.Name), domainName),
		Content:  content,
		TTL:      effectiveTTLInt(input.TTL),
		Priority: input.Priority,
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
	if strings.TrimSpace(updated.Type) == "" {
		return provider.DNSRecord{}, fmt.Errorf("record type is required")
	}
	if strings.TrimSpace(updated.Content) == "" {
		return provider.DNSRecord{}, fmt.Errorf("record content is required")
	}
	updated.TTL = effectiveTTLInt(updated.TTL)
	return updated, nil
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
	if !strings.EqualFold(strings.TrimSpace(recordType), "MX") {
		return strings.TrimSpace(value), nil
	}
	parts := strings.Fields(strings.TrimSpace(value))
	if len(parts) < 2 {
		return strings.TrimSpace(value), nil
	}
	priorityText := strings.TrimSpace(parts[0])
	priority := 0
	for _, r := range priorityText {
		if r < '0' || r > '9' {
			return strings.TrimSpace(value), nil
		}
		priority = priority*10 + int(r-'0')
	}
	content := strings.Join(parts[1:], " ")
	return content, &priority
}

func findRecordset(items []recordset, name, recordType string) *recordset {
	targetName := normalizeComparableName(name)
	targetType := strings.ToUpper(strings.TrimSpace(recordType))
	for index := range items {
		if normalizeComparableName(items[index].Name) == targetName && strings.ToUpper(strings.TrimSpace(items[index].Type)) == targetType {
			return &items[index]
		}
	}
	return nil
}

func findRecordsetByID(items []recordset, recordsetID string) *recordset {
	trimmed := strings.TrimSpace(recordsetID)
	for index := range items {
		if strings.TrimSpace(items[index].ID) == trimmed {
			return &items[index]
		}
	}
	return nil
}

func findRecordInRecordset(zoneInfo zone, current recordset, rawValue string) *provider.DNSRecord {
	for _, item := range current.Records {
		if strings.TrimSpace(item) != strings.TrimSpace(rawValue) {
			continue
		}
		record := mapRecord(zoneInfo, current, item)
		return &record
	}
	return nil
}

func sameRecordsetIdentity(currentName, currentType, desiredName, desiredType, domainName string) bool {
	return normalizeComparableName(currentName) == normalizeComparableName(fqdnRecordName(desiredName, domainName)) && strings.EqualFold(strings.TrimSpace(currentType), strings.TrimSpace(desiredType))
}

func encodeRecordID(recordsetID, rawValue string) string {
	return strings.TrimSpace(recordsetID) + "|" + base64.RawURLEncoding.EncodeToString([]byte(strings.TrimSpace(rawValue)))
}

func decodeRecordID(recordID string) (string, string, error) {
	parts := strings.Split(strings.TrimSpace(recordID), "|")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid huawei record id")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", "", fmt.Errorf("invalid huawei record id")
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(string(decoded)), nil
}

func cloneStrings(items []string) []string {
	cloned := make([]string, len(items))
	copy(cloned, items)
	return cloned
}

func effectiveTTL(current recordset) int {
	return effectiveTTLInt(current.TTL)
}

func effectiveTTLFromRecord(current recordset, desiredTTL int) int {
	if desiredTTL > 0 {
		return desiredTTL
	}
	return effectiveTTL(current)
}

func effectiveTTLInt(ttl int) int {
	if ttl > 0 {
		return ttl
	}
	return defaultTTL
}

func normalizeEndpoint(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if strings.Contains(trimmed, "://") {
		return trimmed
	}
	return "https://" + trimmed
}

func fqdnRecordName(name, domainName string) string {
	trimmedName := strings.TrimSpace(name)
	trimmedDomain := strings.TrimSuffix(strings.TrimSpace(domainName), ".")
	if trimmedName == "" || trimmedName == "@" {
		if trimmedDomain == "" {
			return "@"
		}
		return trimmedDomain + "."
	}
	if trimmedDomain == "" {
		return strings.TrimSuffix(trimmedName, ".") + "."
	}
	candidate := strings.TrimSuffix(trimmedName, ".")
	if strings.EqualFold(candidate, trimmedDomain) {
		return trimmedDomain + "."
	}
	suffix := "." + strings.ToLower(trimmedDomain)
	if strings.HasSuffix(strings.ToLower(candidate), suffix) {
		return strings.TrimSuffix(candidate, ".") + "."
	}
	return candidate + "." + trimmedDomain + "."
}

func normalizeRecordName(name, domainName string) string {
	trimmedName := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(name)), ".")
	trimmedDomain := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(domainName)), ".")
	if trimmedName == "" || trimmedName == "@" {
		return "@"
	}
	if trimmedName == trimmedDomain {
		return "@"
	}
	suffix := "." + trimmedDomain
	if strings.HasSuffix(trimmedName, suffix) {
		short := strings.TrimSuffix(trimmedName, suffix)
		short = strings.TrimSuffix(short, ".")
		if short == "" {
			return "@"
		}
		return short
	}
	return strings.TrimSpace(strings.TrimSuffix(name, "."))
}

func normalizeComparableName(name string) string {
	trimmed := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(name)), ".")
	if trimmed == "" {
		return "@"
	}
	return trimmed
}
