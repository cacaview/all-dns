package alidns

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"dns-hub/server/internal/provider"
)

const (
	endpoint = "https://alidns.aliyuncs.com/"
	version  = "2015-01-09"
	pageSize = 100
)

type Adapter struct {
	accessKeyID     string
	accessKeySecret string
	client          *http.Client
}

type domainsResponse struct {
	RequestID  string     `json:"RequestId"`
	TotalCount int        `json:"TotalCount"`
	PageNumber int        `json:"PageNumber"`
	PageSize   int        `json:"PageSize"`
	Domains    domainList `json:"Domains"`
}

type domainList struct {
	Domain []domainItem `json:"Domain"`
}

type domainItem struct {
	DomainID   string `json:"DomainId"`
	DomainName string `json:"DomainName"`
}

type recordsResponse struct {
	RequestID     string     `json:"RequestId"`
	TotalCount    int        `json:"TotalCount"`
	PageNumber    int        `json:"PageNumber"`
	PageSize      int        `json:"PageSize"`
	DomainRecords recordList `json:"DomainRecords"`
}

type recordList struct {
	Record []domainRecord `json:"Record"`
}

type domainRecord struct {
	RecordID string `json:"RecordId"`
	RR       string `json:"RR"`
	Type     string `json:"Type"`
	Value    string `json:"Value"`
	TTL      int    `json:"TTL"`
	Priority int    `json:"Priority"`
}

type recordMutationResponse struct {
	RequestID string `json:"RequestId"`
	RecordID  string `json:"RecordId"`
}

type apiErrorResponse struct {
	RequestID string `json:"RequestId"`
	Code      string `json:"Code"`
	Message   string `json:"Message"`
}

func init() {
	provider.MustRegister("alidns", func(config map[string]any) (provider.DNSProvider, error) {
		return New(config)
	})
	provider.MustRegisterDescriptor(provider.Descriptor{
		Key:         "alidns",
		Label:       "阿里云 DNS",
		Description: "Alibaba Cloud DNS API",
		Fields: []provider.FieldSpec{
			{Key: "access_key_id", Label: "Access Key ID", Type: provider.FieldTypeText, Required: true, Placeholder: "LTAI...", HelpText: "使用阿里云 AccessKey ID"},
			{Key: "access_key_secret", Label: "Access Key Secret", Type: provider.FieldTypePassword, Required: true, Placeholder: "AccessKey Secret", HelpText: "使用阿里云 AccessKey Secret"},
		},
		SampleConfig: map[string]any{
			"access_key_id":     "",
			"access_key_secret": "",
		},
	})
}

func New(config map[string]any) (*Adapter, error) {
	accessKeyID, _ := config["access_key_id"].(string)
	accessKeySecret, _ := config["access_key_secret"].(string)
	accessKeyID = strings.TrimSpace(accessKeyID)
	accessKeySecret = strings.TrimSpace(accessKeySecret)
	if accessKeyID == "" {
		return nil, fmt.Errorf("alidns access_key_id is required")
	}
	if accessKeySecret == "" {
		return nil, fmt.Errorf("alidns access_key_secret is required")
	}
	return &Adapter{
		accessKeyID:     accessKeyID,
		accessKeySecret: accessKeySecret,
		client:          &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (a *Adapter) Name() string {
	return "alidns"
}

func (a *Adapter) Validate(ctx context.Context) (*provider.ValidationResult, error) {
	_, err := a.ListDomains(ctx)
	if err != nil {
		return nil, err
	}
	return &provider.ValidationResult{OK: true, Message: "alidns credentials are valid", CheckedAt: time.Now().UTC()}, nil
}

func (a *Adapter) ListDomains(ctx context.Context) ([]provider.Domain, error) {
	items := make([]provider.Domain, 0)
	page := 1
	for {
		params := url.Values{}
		params.Set("PageNumber", fmt.Sprintf("%d", page))
		params.Set("PageSize", fmt.Sprintf("%d", pageSize))
		var response domainsResponse
		if err := a.request(ctx, "DescribeDomains", params, &response); err != nil {
			return nil, err
		}
		for _, item := range response.Domains.Domain {
			name := strings.TrimSpace(item.DomainName)
			if name == "" {
				continue
			}
			items = append(items, provider.Domain{ZoneID: name, Name: name, Provider: a.Name()})
		}
		if len(response.Domains.Domain) == 0 || len(response.Domains.Domain) < pageSize {
			break
		}
		if response.TotalCount > 0 && len(items) >= response.TotalCount {
			break
		}
		page++
	}
	return items, nil
}

func (a *Adapter) ListRecords(ctx context.Context, zoneID string) ([]provider.DNSRecord, error) {
	domainName := strings.TrimSpace(zoneID)
	if domainName == "" {
		return nil, fmt.Errorf("domain name is required")
	}
	items := make([]provider.DNSRecord, 0)
	page := 1
	for {
		params := url.Values{}
		params.Set("DomainName", domainName)
		params.Set("PageNumber", fmt.Sprintf("%d", page))
		params.Set("PageSize", fmt.Sprintf("%d", pageSize))
		var response recordsResponse
		if err := a.request(ctx, "DescribeDomainRecords", params, &response); err != nil {
			return nil, err
		}
		for _, item := range response.DomainRecords.Record {
			items = append(items, mapRecord(item, domainName))
		}
		if len(response.DomainRecords.Record) == 0 || len(response.DomainRecords.Record) < pageSize {
			break
		}
		if response.TotalCount > 0 && len(items) >= response.TotalCount {
			break
		}
		page++
	}
	return items, nil
}

func (a *Adapter) UpsertRecord(ctx context.Context, zoneID string, input provider.RecordMutation) (*provider.DNSRecord, error) {
	domainName := strings.TrimSpace(zoneID)
	if domainName == "" {
		return nil, fmt.Errorf("domain name is required")
	}
	trimmedID := strings.TrimSpace(input.ID)
	if trimmedID == "" {
		params, err := createParams(input, domainName)
		if err != nil {
			return nil, err
		}
		var response recordMutationResponse
		if err := a.request(ctx, "AddDomainRecord", params, &response); err != nil {
			return nil, err
		}
		return a.fetchRecordByID(ctx, domainName, strings.TrimSpace(response.RecordID))
	}
	records, err := a.ListRecords(ctx, domainName)
	if err != nil {
		return nil, err
	}
	current := findRecord(records, trimmedID)
	if current == nil {
		return nil, fmt.Errorf("record %s not found", trimmedID)
	}
	params, err := updateParams(*current, input, domainName)
	if err != nil {
		return nil, err
	}
	var response recordMutationResponse
	if err := a.request(ctx, "UpdateDomainRecord", params, &response); err != nil {
		return nil, err
	}
	return a.fetchRecordByID(ctx, domainName, trimmedID)
}

func (a *Adapter) DeleteRecord(ctx context.Context, zoneID string, recordID string) error {
	if strings.TrimSpace(zoneID) == "" {
		return fmt.Errorf("domain name is required")
	}
	trimmedID := strings.TrimSpace(recordID)
	if trimmedID == "" {
		return fmt.Errorf("record id is required")
	}
	params := url.Values{}
	params.Set("RecordId", trimmedID)
	return a.request(ctx, "DeleteDomainRecord", params, nil)
}

func (a *Adapter) ExportConfig() map[string]any {
	return map[string]any{
		"access_key_id":     a.accessKeyID,
		"access_key_secret": a.accessKeySecret,
	}
}

func (a *Adapter) fetchRecordByID(ctx context.Context, domainName, recordID string) (*provider.DNSRecord, error) {
	records, err := a.ListRecords(ctx, domainName)
	if err != nil {
		return nil, err
	}
	record := findRecord(records, recordID)
	if record == nil {
		return nil, fmt.Errorf("record %s not found", recordID)
	}
	return record, nil
}

func createParams(input provider.RecordMutation, domainName string) (url.Values, error) {
	recordType := strings.ToUpper(strings.TrimSpace(input.Type))
	if recordType == "" {
		return nil, fmt.Errorf("record type is required")
	}
	content := strings.TrimSpace(input.Content)
	if content == "" {
		return nil, fmt.Errorf("record content is required")
	}
	params := url.Values{}
	params.Set("DomainName", domainName)
	params.Set("RR", normalizeRecordName(input.Name, domainName))
	params.Set("Type", recordType)
	params.Set("Value", content)
	if input.TTL > 0 {
		params.Set("TTL", fmt.Sprintf("%d", input.TTL))
	}
	if input.Priority != nil && usesPriority(recordType) {
		params.Set("Priority", fmt.Sprintf("%d", *input.Priority))
	}
	return params, nil
}

func updateParams(current provider.DNSRecord, input provider.RecordMutation, domainName string) (url.Values, error) {
	updated, err := mergeRecord(current, input, domainName)
	if err != nil {
		return nil, err
	}
	params := url.Values{}
	params.Set("RecordId", strings.TrimSpace(updated.ID))
	params.Set("RR", normalizeRecordName(updated.Name, domainName))
	params.Set("Type", strings.ToUpper(strings.TrimSpace(updated.Type)))
	params.Set("Value", strings.TrimSpace(updated.Content))
	if updated.TTL > 0 {
		params.Set("TTL", fmt.Sprintf("%d", updated.TTL))
	}
	if updated.Priority != nil && usesPriority(updated.Type) {
		params.Set("Priority", fmt.Sprintf("%d", *updated.Priority))
	}
	return params, nil
}

func mergeRecord(current provider.DNSRecord, input provider.RecordMutation, domainName string) (provider.DNSRecord, error) {
	updated := current
	if strings.TrimSpace(input.Type) != "" {
		updated.Type = strings.ToUpper(strings.TrimSpace(input.Type))
	}
	if input.Name != "" {
		updated.Name = normalizeRecordName(input.Name, domainName)
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
	return updated, nil
}

func mapRecord(item domainRecord, domainName string) provider.DNSRecord {
	record := provider.DNSRecord{
		ID:      strings.TrimSpace(item.RecordID),
		Type:    strings.ToUpper(strings.TrimSpace(item.Type)),
		Name:    normalizeRecordName(item.RR, domainName),
		Content: strings.TrimSpace(item.Value),
		TTL:     item.TTL,
	}
	if usesPriority(record.Type) && item.Priority > 0 {
		priority := item.Priority
		record.Priority = &priority
	}
	if record.TTL <= 0 {
		record.TTL = 600
	}
	return record
}

func findRecord(records []provider.DNSRecord, recordID string) *provider.DNSRecord {
	for index := range records {
		if strings.TrimSpace(records[index].ID) == recordID {
			return &records[index]
		}
	}
	return nil
}

func usesPriority(recordType string) bool {
	switch strings.ToUpper(strings.TrimSpace(recordType)) {
	case "MX":
		return true
	default:
		return false
	}
}

func normalizeRecordName(name, domainName string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" || trimmed == "@" {
		return "@"
	}
	trimmedDomain := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(domainName)), ".")
	candidate := strings.TrimSuffix(strings.ToLower(trimmed), ".")
	if candidate == trimmedDomain {
		return "@"
	}
	suffix := "." + trimmedDomain
	if strings.HasSuffix(candidate, suffix) {
		short := strings.TrimSuffix(candidate, suffix)
		short = strings.TrimSuffix(short, ".")
		if short == "" {
			return "@"
		}
		return short
	}
	return trimmed
}

func (a *Adapter) request(ctx context.Context, action string, params url.Values, target any) error {
	query := a.commonParams(action)
	for key, values := range params {
		for _, value := range values {
			query.Add(key, value)
		}
	}
	query.Set("Signature", a.sign(http.MethodGet, query))
	requestURL := endpoint + "?" + query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return fmt.Errorf("build alidns request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	response, err := a.client.Do(request)
	if err != nil {
		return fmt.Errorf("call alidns api: %w", err)
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("read alidns response: %w", err)
	}
	if response.StatusCode >= 400 {
		return fmt.Errorf("alidns api returned %s: %s", response.Status, strings.TrimSpace(string(payload)))
	}
	var apiErr apiErrorResponse
	if err := json.Unmarshal(payload, &apiErr); err == nil && strings.TrimSpace(apiErr.Code) != "" {
		return fmt.Errorf("alidns api error %s: %s", strings.TrimSpace(apiErr.Code), strings.TrimSpace(apiErr.Message))
	}
	if target == nil || len(payload) == 0 {
		return nil
	}
	if err := json.Unmarshal(payload, target); err != nil {
		return fmt.Errorf("decode alidns response: %w", err)
	}
	return nil
}

func (a *Adapter) commonParams(action string) url.Values {
	params := url.Values{}
	params.Set("Action", action)
	params.Set("Format", "JSON")
	params.Set("Version", version)
	params.Set("AccessKeyId", a.accessKeyID)
	params.Set("SignatureMethod", "HMAC-SHA1")
	params.Set("SignatureVersion", "1.0")
	params.Set("SignatureNonce", signatureNonce())
	params.Set("Timestamp", time.Now().UTC().Format("2006-01-02T15:04:05Z"))
	return params
}

func (a *Adapter) sign(method string, params url.Values) string {
	canonicalized := canonicalizedQuery(params)
	stringToSign := strings.ToUpper(strings.TrimSpace(method)) + "&%2F&" + percentEncode(canonicalized)
	mac := hmac.New(sha1.New, []byte(a.accessKeySecret+"&"))
	_, _ = mac.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func canonicalizedQuery(params url.Values) string {
	keys := make([]string, 0, len(params))
	for key := range params {
		if strings.EqualFold(key, "Signature") {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		values := append([]string(nil), params[key]...)
		sort.Strings(values)
		for _, value := range values {
			parts = append(parts, percentEncode(key)+"="+percentEncode(value))
		}
	}
	return strings.Join(parts, "&")
}

func percentEncode(value string) string {
	escaped := url.QueryEscape(value)
	escaped = strings.ReplaceAll(escaped, "+", "%20")
	escaped = strings.ReplaceAll(escaped, "*", "%2A")
	escaped = strings.ReplaceAll(escaped, "%7E", "~")
	return escaped
}

func signatureNonce() string {
	buffer := make([]byte, 8)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buffer)
}
