package dnspod

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
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

const (
	defaultEndpoint   = "https://dnspod.intl.tencentcloudapi.com"
	serviceName       = "dnspod"
	apiVersion        = "2021-03-23"
	defaultRecordLine = "默认"
	pageLimit         = 100
)

type Adapter struct {
	secretID     string
	secretKey    string
	endpoint     string
	host         string
	recordLine   string
	recordLineID string
	client       *http.Client
}

type apiError struct {
	Code    string `json:"Code"`
	Message string `json:"Message"`
}

type apiEnvelope struct {
	Response struct {
		RequestID string    `json:"RequestId"`
		Error     *apiError `json:"Error"`
	} `json:"Response"`
}

type domainsResponse struct {
	Response struct {
		DomainCountInfo struct {
			TotalCount int `json:"TotalCount"`
		} `json:"DomainCountInfo"`
		DomainList []domainItem `json:"DomainList"`
		RequestID  string       `json:"RequestId"`
		Error      *apiError    `json:"Error"`
	} `json:"Response"`
}

type domainItem struct {
	DomainID uint64 `json:"DomainId"`
	Name     string `json:"Name"`
	Domain   string `json:"Domain"`
}

type recordsResponse struct {
	Response struct {
		RecordCountInfo struct {
			TotalCount int `json:"TotalCount"`
		} `json:"RecordCountInfo"`
		RecordList []domainRecord `json:"RecordList"`
		RequestID  string         `json:"RequestId"`
		Error      *apiError      `json:"Error"`
	} `json:"Response"`
}

type recordMutationResponse struct {
	Response struct {
		RecordID  uint64    `json:"RecordId"`
		RequestID string    `json:"RequestId"`
		Error     *apiError `json:"Error"`
	} `json:"Response"`
}

type domainRecord struct {
	RecordID uint64 `json:"RecordId"`
	Name     string `json:"Name"`
	Type     string `json:"Type"`
	Value    string `json:"Value"`
	TTL      uint64 `json:"TTL"`
	MX       uint64 `json:"MX"`
	Line     string `json:"Line"`
	LineID   string `json:"LineId"`
}

func init() {
	provider.MustRegister("dnspod", func(config map[string]any) (provider.DNSProvider, error) {
		return New(config)
	})
	provider.MustRegisterDescriptor(provider.Descriptor{
		Key:         "dnspod",
		Label:       "腾讯云 DNSPod",
		Description: "Tencent Cloud DNSPod API",
		Fields: []provider.FieldSpec{
			{Key: "secret_id", Label: "Secret ID", Type: provider.FieldTypeText, Required: true, Placeholder: "AKID...", HelpText: "使用腾讯云 API SecretId"},
			{Key: "secret_key", Label: "Secret Key", Type: provider.FieldTypePassword, Required: true, Placeholder: "SecretKey", HelpText: "使用腾讯云 API SecretKey"},
			{Key: "record_line", Label: "Record Line", Type: provider.FieldTypeText, Required: false, Placeholder: defaultRecordLine, HelpText: "可选，默认使用“默认”线路"},
			{Key: "record_line_id", Label: "Record Line ID", Type: provider.FieldTypeText, Required: false, Placeholder: "0", HelpText: "可选，配置后优先于 Record Line"},
			{Key: "endpoint", Label: "API Endpoint", Type: provider.FieldTypeText, Required: false, Placeholder: defaultEndpoint, HelpText: "可选，默认使用国际站 DNSPod API Endpoint"},
		},
		SampleConfig: map[string]any{
			"secret_id":      "",
			"secret_key":     "",
			"record_line":    defaultRecordLine,
			"record_line_id": "",
			"endpoint":       defaultEndpoint,
		},
	})
}

func New(config map[string]any) (*Adapter, error) {
	secretID, _ := config["secret_id"].(string)
	secretKey, _ := config["secret_key"].(string)
	recordLine, _ := config["record_line"].(string)
	recordLineID, _ := config["record_line_id"].(string)
	endpoint, _ := config["endpoint"].(string)

	secretID = strings.TrimSpace(secretID)
	secretKey = strings.TrimSpace(secretKey)
	recordLine = strings.TrimSpace(recordLine)
	recordLineID = strings.TrimSpace(recordLineID)
	endpoint = strings.TrimSpace(endpoint)

	if secretID == "" {
		return nil, fmt.Errorf("dnspod secret_id is required")
	}
	if secretKey == "" {
		return nil, fmt.Errorf("dnspod secret_key is required")
	}
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	if !strings.Contains(endpoint, "://") {
		endpoint = "https://" + endpoint
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse dnspod endpoint: %w", err)
	}
	if strings.TrimSpace(parsed.Scheme) == "" || strings.TrimSpace(parsed.Host) == "" {
		return nil, fmt.Errorf("dnspod endpoint is invalid")
	}

	return &Adapter{
		secretID:     secretID,
		secretKey:    secretKey,
		endpoint:     strings.TrimRight(endpoint, "/"),
		host:         parsed.Host,
		recordLine:   recordLine,
		recordLineID: recordLineID,
		client:       &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (a *Adapter) Name() string {
	return "dnspod"
}

func (a *Adapter) Validate(ctx context.Context) (*provider.ValidationResult, error) {
	_, err := a.ListDomains(ctx)
	if err != nil {
		return nil, err
	}
	return &provider.ValidationResult{OK: true, Message: "dnspod credentials are valid", CheckedAt: time.Now().UTC()}, nil
}

func (a *Adapter) ListDomains(ctx context.Context) ([]provider.Domain, error) {
	items := make([]provider.Domain, 0)
	offset := 0
	for {
		var response domainsResponse
		if err := a.request(ctx, "DescribeDomainList", map[string]any{"Offset": offset, "Limit": pageLimit}, &response); err != nil {
			return nil, err
		}
		for _, item := range response.Response.DomainList {
			name := firstNonEmpty(strings.TrimSpace(item.Name), strings.TrimSpace(item.Domain))
			if name == "" {
				continue
			}
			items = append(items, provider.Domain{ZoneID: name, Name: name, Provider: a.Name()})
		}
		if len(response.Response.DomainList) == 0 || len(response.Response.DomainList) < pageLimit {
			break
		}
		if response.Response.DomainCountInfo.TotalCount > 0 && len(items) >= response.Response.DomainCountInfo.TotalCount {
			break
		}
		offset += len(response.Response.DomainList)
	}
	return items, nil
}

func (a *Adapter) ListRecords(ctx context.Context, zoneID string) ([]provider.DNSRecord, error) {
	domainName := strings.TrimSpace(zoneID)
	if domainName == "" {
		return nil, fmt.Errorf("domain name is required")
	}
	records, err := a.listRawRecords(ctx, domainName)
	if err != nil {
		return nil, err
	}
	items := make([]provider.DNSRecord, 0, len(records))
	for _, item := range records {
		items = append(items, mapRecord(item, domainName))
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
		payload, err := a.createPayload(domainName, input)
		if err != nil {
			return nil, err
		}
		var response recordMutationResponse
		if err := a.request(ctx, "CreateRecord", payload, &response); err != nil {
			return nil, err
		}
		return a.fetchRecordByID(ctx, domainName, strconv.FormatUint(response.Response.RecordID, 10))
	}

	records, err := a.listRawRecords(ctx, domainName)
	if err != nil {
		return nil, err
	}
	current := findRawRecord(records, trimmedID)
	if current == nil {
		return nil, fmt.Errorf("record %s not found", trimmedID)
	}
	payload, err := a.updatePayload(domainName, *current, input)
	if err != nil {
		return nil, err
	}
	var response recordMutationResponse
	if err := a.request(ctx, "ModifyRecord", payload, &response); err != nil {
		return nil, err
	}
	return a.fetchRecordByID(ctx, domainName, trimmedID)
}

func (a *Adapter) DeleteRecord(ctx context.Context, zoneID string, recordID string) error {
	domainName := strings.TrimSpace(zoneID)
	trimmedID := strings.TrimSpace(recordID)
	if domainName == "" {
		return fmt.Errorf("domain name is required")
	}
	if trimmedID == "" {
		return fmt.Errorf("record id is required")
	}
	payload := map[string]any{
		"Domain":   domainName,
		"RecordId": recordIDValue(trimmedID),
	}
	return a.request(ctx, "DeleteRecord", payload, nil)
}

func (a *Adapter) ExportConfig() map[string]any {
	config := map[string]any{
		"secret_id":  a.secretID,
		"secret_key": a.secretKey,
		"endpoint":   a.endpoint,
	}
	if a.recordLine != "" {
		config["record_line"] = a.recordLine
	}
	if a.recordLineID != "" {
		config["record_line_id"] = a.recordLineID
	}
	return config
}

func (a *Adapter) listRawRecords(ctx context.Context, domainName string) ([]domainRecord, error) {
	items := make([]domainRecord, 0)
	offset := 0
	for {
		var response recordsResponse
		payload := map[string]any{
			"Domain": domainName,
			"Offset": offset,
			"Limit":  pageLimit,
		}
		if err := a.request(ctx, "DescribeRecordList", payload, &response); err != nil {
			return nil, err
		}
		items = append(items, response.Response.RecordList...)
		if len(response.Response.RecordList) == 0 || len(response.Response.RecordList) < pageLimit {
			break
		}
		if response.Response.RecordCountInfo.TotalCount > 0 && len(items) >= response.Response.RecordCountInfo.TotalCount {
			break
		}
		offset += len(response.Response.RecordList)
	}
	return items, nil
}

func (a *Adapter) fetchRecordByID(ctx context.Context, domainName, recordID string) (*provider.DNSRecord, error) {
	records, err := a.listRawRecords(ctx, domainName)
	if err != nil {
		return nil, err
	}
	record := findRawRecord(records, recordID)
	if record == nil {
		return nil, fmt.Errorf("record %s not found", recordID)
	}
	mapped := mapRecord(*record, domainName)
	return &mapped, nil
}

func (a *Adapter) createPayload(domainName string, input provider.RecordMutation) (map[string]any, error) {
	recordType := strings.ToUpper(strings.TrimSpace(input.Type))
	if recordType == "" {
		return nil, fmt.Errorf("record type is required")
	}
	content := strings.TrimSpace(input.Content)
	if content == "" {
		return nil, fmt.Errorf("record content is required")
	}
	payload := map[string]any{
		"Domain":     domainName,
		"SubDomain":  normalizeRecordName(input.Name, domainName),
		"RecordType": recordType,
		"Value":      content,
	}
	a.applyCreateRecordLine(payload)
	if input.TTL > 0 {
		payload["TTL"] = input.TTL
	}
	if input.Priority != nil && usesPriority(recordType) {
		payload["MX"] = *input.Priority
	}
	return payload, nil
}

func (a *Adapter) updatePayload(domainName string, current domainRecord, input provider.RecordMutation) (map[string]any, error) {
	recordType := firstNonEmpty(strings.ToUpper(strings.TrimSpace(input.Type)), strings.ToUpper(strings.TrimSpace(current.Type)))
	if recordType == "" {
		return nil, fmt.Errorf("record type is required")
	}
	name := normalizeRecordName(current.Name, domainName)
	if strings.TrimSpace(input.Name) != "" {
		name = normalizeRecordName(input.Name, domainName)
	}
	content := firstNonEmpty(strings.TrimSpace(input.Content), strings.TrimSpace(current.Value))
	if content == "" {
		return nil, fmt.Errorf("record content is required")
	}
	ttl := int(current.TTL)
	if ttl <= 0 {
		ttl = 600
	}
	if input.TTL > 0 {
		ttl = input.TTL
	}
	payload := map[string]any{
		"Domain":     domainName,
		"RecordId":   recordIDValue(strconv.FormatUint(current.RecordID, 10)),
		"SubDomain":  name,
		"RecordType": recordType,
		"Value":      content,
		"TTL":        ttl,
	}
	a.applyUpdateRecordLine(payload, current)
	if usesPriority(recordType) {
		priority := priorityFromRaw(current)
		if input.Priority != nil {
			priority = input.Priority
		}
		if priority != nil {
			payload["MX"] = *priority
		}
	}
	return payload, nil
}

func (a *Adapter) applyCreateRecordLine(payload map[string]any) {
	if a.recordLineID != "" {
		payload["RecordLineId"] = a.recordLineID
		return
	}
	payload["RecordLine"] = firstNonEmpty(a.recordLine, defaultRecordLine)
}

func (a *Adapter) applyUpdateRecordLine(payload map[string]any, current domainRecord) {
	if a.recordLineID != "" {
		payload["RecordLineId"] = a.recordLineID
		return
	}
	if a.recordLine != "" {
		payload["RecordLine"] = a.recordLine
		return
	}
	if strings.TrimSpace(current.LineID) != "" {
		payload["RecordLineId"] = strings.TrimSpace(current.LineID)
		return
	}
	payload["RecordLine"] = firstNonEmpty(strings.TrimSpace(current.Line), defaultRecordLine)
}

func mapRecord(item domainRecord, domainName string) provider.DNSRecord {
	record := provider.DNSRecord{
		ID:      strconv.FormatUint(item.RecordID, 10),
		Type:    strings.ToUpper(strings.TrimSpace(item.Type)),
		Name:    normalizeRecordName(item.Name, domainName),
		Content: strings.TrimSpace(item.Value),
		TTL:     int(item.TTL),
	}
	if record.TTL <= 0 {
		record.TTL = 600
	}
	if usesPriority(record.Type) && item.MX > 0 {
		priority := int(item.MX)
		record.Priority = &priority
	}
	return record
}

func findRawRecord(records []domainRecord, recordID string) *domainRecord {
	for index := range records {
		if strconv.FormatUint(records[index].RecordID, 10) == recordID {
			return &records[index]
		}
	}
	return nil
}

func priorityFromRaw(record domainRecord) *int {
	if record.MX == 0 {
		return nil
	}
	priority := int(record.MX)
	return &priority
}

func usesPriority(recordType string) bool {
	return strings.EqualFold(strings.TrimSpace(recordType), "MX")
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

func recordIDValue(recordID string) any {
	if value, err := strconv.ParseUint(strings.TrimSpace(recordID), 10, 64); err == nil {
		return value
	}
	return strings.TrimSpace(recordID)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (a *Adapter) request(ctx context.Context, action string, body any, target any) error {
	if body == nil {
		body = map[string]any{}
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal dnspod request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, a.endpoint, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build dnspod request: %w", err)
	}
	request.Host = a.host
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Host", a.host)
	request.Header.Set("X-TC-Action", action)
	request.Header.Set("X-TC-Version", apiVersion)
	timestamp := time.Now().UTC().Unix()
	request.Header.Set("X-TC-Timestamp", strconv.FormatInt(timestamp, 10))
	request.Header.Set("Authorization", a.authorization(action, timestamp, payload))

	response, err := a.client.Do(request)
	if err != nil {
		return fmt.Errorf("call dnspod api: %w", err)
	}
	defer response.Body.Close()

	responsePayload, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("read dnspod response: %w", err)
	}
	if response.StatusCode >= 400 {
		return fmt.Errorf("dnspod api returned %s: %s", response.Status, strings.TrimSpace(string(responsePayload)))
	}
	var envelope apiEnvelope
	if err := json.Unmarshal(responsePayload, &envelope); err == nil && envelope.Response.Error != nil {
		return fmt.Errorf("dnspod api error %s: %s", strings.TrimSpace(envelope.Response.Error.Code), strings.TrimSpace(envelope.Response.Error.Message))
	}
	if target == nil || len(responsePayload) == 0 {
		return nil
	}
	if err := json.Unmarshal(responsePayload, target); err != nil {
		return fmt.Errorf("decode dnspod response: %w", err)
	}
	return nil
}

func (a *Adapter) authorization(action string, timestamp int64, payload []byte) string {
	date := time.Unix(timestamp, 0).UTC().Format("2006-01-02")
	hashedPayload := sha256Hex(payload)
	canonicalHeaders := "content-type:application/json; charset=utf-8\n" +
		"host:" + a.host + "\n" +
		"x-tc-action:" + strings.ToLower(strings.TrimSpace(action)) + "\n"
	signedHeaders := "content-type;host;x-tc-action"
	canonicalRequest := strings.Join([]string{
		http.MethodPost,
		"/",
		"",
		canonicalHeaders,
		signedHeaders,
		hashedPayload,
	}, "\n")
	credentialScope := date + "/" + serviceName + "/tc3_request"
	stringToSign := strings.Join([]string{
		"TC3-HMAC-SHA256",
		strconv.FormatInt(timestamp, 10),
		credentialScope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")
	secretDate := hmacSHA256([]byte("TC3"+a.secretKey), date)
	secretService := hmacSHA256(secretDate, serviceName)
	secretSigning := hmacSHA256(secretService, "tc3_request")
	signature := hex.EncodeToString(hmacSHA256(secretSigning, stringToSign))
	return fmt.Sprintf(
		"TC3-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		a.secretID,
		credentialScope,
		signedHeaders,
		signature,
	)
}

func sha256Hex(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func hmacSHA256(key []byte, value string) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(value))
	return mac.Sum(nil)
}
