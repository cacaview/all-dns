package gcp

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
	"golang.org/x/oauth2/jwt"
)

const (
	defaultEndpoint = "https://dns.googleapis.com/dns/v1"
	defaultTokenURL = "https://oauth2.googleapis.com/token"
	defaultTTL      = 300
	pageLimit       = 100
)

type Adapter struct {
	projectID   string
	clientEmail string
	privateKey  string
	tokenURL    string
	endpoint    string
	client      *http.Client
}

type managedZonesResponse struct {
	ManagedZones  []managedZone `json:"managedZones"`
	NextPageToken string        `json:"nextPageToken"`
}

type managedZone struct {
	Name    string `json:"name"`
	DNSName string `json:"dnsName"`
}

type resourceRecordSetsResponse struct {
	RRsets        []resourceRecordSet `json:"rrsets"`
	NextPageToken string              `json:"nextPageToken"`
}

type resourceRecordSet struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	TTL     int      `json:"ttl"`
	RRDatas []string `json:"rrdatas"`
}

type changeRequest struct {
	Additions []resourceRecordSet `json:"additions,omitempty"`
	Deletions []resourceRecordSet `json:"deletions,omitempty"`
}

func init() {
	provider.MustRegister("gcp", func(config map[string]any) (provider.DNSProvider, error) {
		return New(config)
	})
	provider.MustRegisterDescriptor(provider.Descriptor{
		Key:         "gcp",
		Label:       "Google Cloud DNS",
		Description: "Google Cloud DNS REST API",
		Fields: []provider.FieldSpec{
			{Key: "project_id", Label: "Project ID", Type: provider.FieldTypeText, Required: true, Placeholder: "my-gcp-project", HelpText: "Google Cloud Project ID"},
			{Key: "client_email", Label: "Client Email", Type: provider.FieldTypeText, Required: true, Placeholder: "dns-service-account@project.iam.gserviceaccount.com", HelpText: "服务账号 Client Email"},
			{Key: "private_key", Label: "Private Key", Type: provider.FieldTypePassword, Required: true, Placeholder: "-----BEGIN PRIVATE KEY-----", HelpText: "服务账号私钥，支持粘贴包含 \\n 的 JSON 值"},
			{Key: "token_url", Label: "Token URL", Type: provider.FieldTypeText, Required: false, Placeholder: defaultTokenURL, HelpText: "可选，默认使用 Google OAuth token endpoint"},
			{Key: "endpoint", Label: "API Endpoint", Type: provider.FieldTypeText, Required: false, Placeholder: defaultEndpoint, HelpText: "可选，默认使用 Google Cloud DNS API Endpoint"},
		},
		SampleConfig: map[string]any{
			"project_id":   "demo-project",
			"client_email": "",
			"private_key":  "",
			"token_url":    defaultTokenURL,
			"endpoint":     defaultEndpoint,
		},
	})
}

func New(config map[string]any) (*Adapter, error) {
	projectID, _ := config["project_id"].(string)
	clientEmail, _ := config["client_email"].(string)
	privateKey, _ := config["private_key"].(string)
	tokenURL, _ := config["token_url"].(string)
	endpoint, _ := config["endpoint"].(string)

	projectID = strings.TrimSpace(projectID)
	clientEmail = strings.TrimSpace(clientEmail)
	privateKey = normalizePrivateKey(privateKey)
	tokenURL = normalizeEndpoint(tokenURL)
	endpoint = normalizeEndpoint(endpoint)

	if projectID == "" {
		return nil, fmt.Errorf("gcp project_id is required")
	}
	if clientEmail == "" {
		return nil, fmt.Errorf("gcp client_email is required")
	}
	if privateKey == "" {
		return nil, fmt.Errorf("gcp private_key is required")
	}
	if tokenURL == "" {
		tokenURL = defaultTokenURL
	}
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	if _, err := url.Parse(tokenURL); err != nil {
		return nil, fmt.Errorf("parse gcp token_url: %w", err)
	}
	if _, err := url.Parse(endpoint); err != nil {
		return nil, fmt.Errorf("parse gcp endpoint: %w", err)
	}

	return &Adapter{
		projectID:   projectID,
		clientEmail: clientEmail,
		privateKey:  privateKey,
		tokenURL:    strings.TrimRight(tokenURL, "/"),
		endpoint:    strings.TrimRight(endpoint, "/"),
		client:      &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (a *Adapter) Name() string {
	return "gcp"
}

func (a *Adapter) Validate(ctx context.Context) (*provider.ValidationResult, error) {
	_, err := a.ListDomains(ctx)
	if err != nil {
		return nil, err
	}
	return &provider.ValidationResult{OK: true, Message: "gcp credentials are valid", CheckedAt: time.Now().UTC()}, nil
}

func (a *Adapter) ListDomains(ctx context.Context) ([]provider.Domain, error) {
	items := make([]provider.Domain, 0)
	pageToken := ""
	for {
		query := url.Values{}
		query.Set("maxResults", strconv.Itoa(pageLimit))
		if pageToken != "" {
			query.Set("pageToken", pageToken)
		}
		var response managedZonesResponse
		path := "/projects/" + url.PathEscape(a.projectID) + "/managedZones"
		if err := a.request(ctx, http.MethodGet, path, query, nil, &response); err != nil {
			return nil, err
		}
		for _, item := range response.ManagedZones {
			zoneID := strings.TrimSpace(item.Name)
			name := strings.TrimSuffix(strings.TrimSpace(item.DNSName), ".")
			if zoneID == "" || name == "" {
				continue
			}
			items = append(items, provider.Domain{ZoneID: zoneID, Name: name, Provider: a.Name()})
		}
		if strings.TrimSpace(response.NextPageToken) == "" {
			break
		}
		pageToken = strings.TrimSpace(response.NextPageToken)
	}
	return items, nil
}

func (a *Adapter) ListRecords(ctx context.Context, zoneID string) ([]provider.DNSRecord, error) {
	zoneInfo, err := a.getManagedZone(ctx, zoneID)
	if err != nil {
		return nil, err
	}
	rrsets, err := a.listRecordSets(ctx, zoneInfo.Name)
	if err != nil {
		return nil, err
	}
	items := make([]provider.DNSRecord, 0)
	for _, item := range rrsets {
		for _, value := range item.RRDatas {
			items = append(items, mapRecord(zoneInfo, item, value))
		}
	}
	return items, nil
}

func (a *Adapter) UpsertRecord(ctx context.Context, zoneID string, input provider.RecordMutation) (*provider.DNSRecord, error) {
	zoneInfo, err := a.getManagedZone(ctx, zoneID)
	if err != nil {
		return nil, err
	}
	rrsets, err := a.listRecordSets(ctx, zoneInfo.Name)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(input.ID) == "" {
		desired, err := mutationToRecord(input, zoneInfo.DNSName)
		if err != nil {
			return nil, err
		}
		return a.upsertDesiredRecord(ctx, zoneInfo, rrsets, desired)
	}

	recordName, recordType, rawValue, err := decodeRecordID(input.ID)
	if err != nil {
		return nil, err
	}
	current := findRecordSet(rrsets, recordName, recordType)
	if current == nil {
		return nil, fmt.Errorf("record %s not found", strings.TrimSpace(input.ID))
	}
	currentRecord := findProviderRecord(zoneInfo, *current, rawValue)
	if currentRecord == nil {
		return nil, fmt.Errorf("record %s not found", strings.TrimSpace(input.ID))
	}
	desired, err := mergeRecord(*currentRecord, input, zoneInfo.DNSName)
	if err != nil {
		return nil, err
	}
	if sameRecordSetIdentity(current.Name, current.Type, desired.Name, desired.Type, zoneInfo.DNSName) {
		if err := a.replaceRecordInSet(ctx, zoneInfo.Name, *current, rawValue, desired); err != nil {
			return nil, err
		}
		return a.readBackRecord(ctx, zoneInfo, desired)
	}
	if err := a.removeRecordFromSet(ctx, zoneInfo.Name, *current, rawValue); err != nil {
		return nil, err
	}
	updated, err := a.listRecordSets(ctx, zoneInfo.Name)
	if err != nil {
		return nil, err
	}
	return a.upsertDesiredRecord(ctx, zoneInfo, updated, desired)
}

func (a *Adapter) DeleteRecord(ctx context.Context, zoneID string, recordID string) error {
	zoneInfo, err := a.getManagedZone(ctx, zoneID)
	if err != nil {
		return err
	}
	recordName, recordType, rawValue, err := decodeRecordID(recordID)
	if err != nil {
		return err
	}
	rrsets, err := a.listRecordSets(ctx, zoneInfo.Name)
	if err != nil {
		return err
	}
	current := findRecordSet(rrsets, recordName, recordType)
	if current == nil {
		return fmt.Errorf("record %s not found", strings.TrimSpace(recordID))
	}
	return a.removeRecordFromSet(ctx, zoneInfo.Name, *current, rawValue)
}

func (a *Adapter) ExportConfig() map[string]any {
	return map[string]any{
		"project_id":   a.projectID,
		"client_email": a.clientEmail,
		"private_key":  a.privateKey,
		"token_url":    a.tokenURL,
		"endpoint":     a.endpoint,
	}
}

func (a *Adapter) getManagedZone(ctx context.Context, zoneID string) (managedZone, error) {
	trimmed := strings.TrimSpace(zoneID)
	if trimmed == "" {
		return managedZone{}, fmt.Errorf("zone id is required")
	}
	var response managedZone
	path := "/projects/" + url.PathEscape(a.projectID) + "/managedZones/" + url.PathEscape(trimmed)
	if err := a.request(ctx, http.MethodGet, path, nil, nil, &response); err != nil {
		return managedZone{}, err
	}
	if strings.TrimSpace(response.Name) == "" {
		return managedZone{}, fmt.Errorf("zone %s not found", trimmed)
	}
	return response, nil
}

func (a *Adapter) listRecordSets(ctx context.Context, zoneID string) ([]resourceRecordSet, error) {
	trimmed := strings.TrimSpace(zoneID)
	if trimmed == "" {
		return nil, fmt.Errorf("zone id is required")
	}
	items := make([]resourceRecordSet, 0)
	pageToken := ""
	for {
		query := url.Values{}
		query.Set("maxResults", strconv.Itoa(pageLimit))
		if pageToken != "" {
			query.Set("pageToken", pageToken)
		}
		var response resourceRecordSetsResponse
		path := "/projects/" + url.PathEscape(a.projectID) + "/managedZones/" + url.PathEscape(trimmed) + "/rrsets"
		if err := a.request(ctx, http.MethodGet, path, query, nil, &response); err != nil {
			return nil, err
		}
		items = append(items, response.RRsets...)
		if strings.TrimSpace(response.NextPageToken) == "" {
			break
		}
		pageToken = strings.TrimSpace(response.NextPageToken)
	}
	return items, nil
}

func (a *Adapter) upsertDesiredRecord(ctx context.Context, zoneInfo managedZone, rrsets []resourceRecordSet, desired provider.DNSRecord) (*provider.DNSRecord, error) {
	recordsetName := fqdnRecordName(desired.Name, zoneInfo.DNSName)
	recordType := strings.ToUpper(strings.TrimSpace(desired.Type))
	rawValue, err := buildRecordValue(recordType, desired.Content, desired.Priority)
	if err != nil {
		return nil, err
	}
	current := findRecordSet(rrsets, recordsetName, recordType)
	if current == nil {
		change := changeRequest{Additions: []resourceRecordSet{{
			Name:    recordsetName,
			Type:    recordType,
			TTL:     effectiveTTLInt(desired.TTL),
			RRDatas: []string{rawValue},
		}}}
		if err := a.submitChange(ctx, zoneInfo.Name, change); err != nil {
			return nil, err
		}
		return a.readBackRecord(ctx, zoneInfo, desired)
	}
	updated := cloneRecordSet(*current)
	updated.Name = recordsetName
	updated.Type = recordType
	updated.TTL = effectiveTTLFromSet(*current, desired.TTL)
	found := false
	for _, item := range updated.RRDatas {
		if strings.TrimSpace(item) == strings.TrimSpace(rawValue) {
			found = true
			break
		}
	}
	if !found {
		updated.RRDatas = append(updated.RRDatas, rawValue)
	}
	change := changeRequest{Additions: []resourceRecordSet{updated}, Deletions: []resourceRecordSet{*current}}
	if err := a.submitChange(ctx, zoneInfo.Name, change); err != nil {
		return nil, err
	}
	return a.readBackRecord(ctx, zoneInfo, desired)
}

func (a *Adapter) replaceRecordInSet(ctx context.Context, zoneID string, current resourceRecordSet, oldValue string, desired provider.DNSRecord) error {
	newValue, err := buildRecordValue(desired.Type, desired.Content, desired.Priority)
	if err != nil {
		return err
	}
	updated := cloneRecordSet(current)
	updated.Name = fqdnRecordName(desired.Name, "")
	updated.Type = strings.ToUpper(strings.TrimSpace(desired.Type))
	updated.TTL = effectiveTTLFromSet(current, desired.TTL)
	replaced := false
	for index := range updated.RRDatas {
		if strings.TrimSpace(updated.RRDatas[index]) != strings.TrimSpace(oldValue) {
			continue
		}
		updated.RRDatas[index] = newValue
		replaced = true
		break
	}
	if !replaced {
		return fmt.Errorf("record %s not found", encodeRecordID(current.Name, current.Type, oldValue))
	}
	return a.submitChange(ctx, zoneID, changeRequest{Additions: []resourceRecordSet{updated}, Deletions: []resourceRecordSet{current}})
}

func (a *Adapter) removeRecordFromSet(ctx context.Context, zoneID string, current resourceRecordSet, rawValue string) error {
	filtered := make([]string, 0, len(current.RRDatas))
	removed := false
	for _, item := range current.RRDatas {
		if strings.TrimSpace(item) == strings.TrimSpace(rawValue) {
			removed = true
			continue
		}
		filtered = append(filtered, item)
	}
	if !removed {
		return fmt.Errorf("record %s not found", encodeRecordID(current.Name, current.Type, rawValue))
	}
	if len(filtered) == 0 {
		return a.submitChange(ctx, zoneID, changeRequest{Deletions: []resourceRecordSet{current}})
	}
	updated := cloneRecordSet(current)
	updated.RRDatas = filtered
	return a.submitChange(ctx, zoneID, changeRequest{Additions: []resourceRecordSet{updated}, Deletions: []resourceRecordSet{current}})
}

func (a *Adapter) readBackRecord(ctx context.Context, zoneInfo managedZone, desired provider.DNSRecord) (*provider.DNSRecord, error) {
	rrsets, err := a.listRecordSets(ctx, zoneInfo.Name)
	if err != nil {
		return nil, err
	}
	targetName := normalizeComparableName(fqdnRecordName(desired.Name, zoneInfo.DNSName))
	targetType := strings.ToUpper(strings.TrimSpace(desired.Type))
	targetValue, err := buildRecordValue(desired.Type, desired.Content, desired.Priority)
	if err != nil {
		return nil, err
	}
	for _, current := range rrsets {
		if normalizeComparableName(current.Name) != targetName || strings.ToUpper(strings.TrimSpace(current.Type)) != targetType {
			continue
		}
		for _, item := range current.RRDatas {
			if strings.TrimSpace(item) != strings.TrimSpace(targetValue) {
				continue
			}
			record := mapRecord(zoneInfo, current, item)
			return &record, nil
		}
	}
	return nil, fmt.Errorf("record not found after gcp change")
}

func (a *Adapter) submitChange(ctx context.Context, zoneID string, payload changeRequest) error {
	path := "/projects/" + url.PathEscape(a.projectID) + "/managedZones/" + url.PathEscape(strings.TrimSpace(zoneID)) + "/changes"
	return a.request(ctx, http.MethodPost, path, nil, payload, nil)
}

func (a *Adapter) request(ctx context.Context, method, path string, query url.Values, body any, target any) error {
	token, err := a.accessToken(ctx)
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
			return fmt.Errorf("marshal gcp request: %w", err)
		}
		reader = bytes.NewBuffer(payload)
	}
	request, err := http.NewRequestWithContext(ctx, method, requestURL, reader)
	if err != nil {
		return fmt.Errorf("build gcp request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := a.client.Do(request)
	if err != nil {
		return fmt.Errorf("call gcp dns api: %w", err)
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("read gcp response: %w", err)
	}
	if response.StatusCode >= 400 {
		return fmt.Errorf("gcp dns api returned %s: %s", response.Status, strings.TrimSpace(string(payload)))
	}
	if target == nil || len(payload) == 0 {
		return nil
	}
	if err := json.Unmarshal(payload, target); err != nil {
		return fmt.Errorf("decode gcp response: %w", err)
	}
	return nil
}

func (a *Adapter) accessToken(ctx context.Context) (string, error) {
	config := &jwt.Config{
		Email:      a.clientEmail,
		PrivateKey: []byte(a.privateKey),
		Scopes: []string{
			"https://www.googleapis.com/auth/ndev.clouddns.readwrite",
		},
		TokenURL: a.tokenURL,
	}
	token, err := config.TokenSource(ctx).Token()
	if err != nil {
		return "", fmt.Errorf("fetch gcp access token: %w", err)
	}
	if strings.TrimSpace(token.AccessToken) == "" {
		return "", fmt.Errorf("gcp access token is empty")
	}
	return token.AccessToken, nil
}

func mapRecord(zoneInfo managedZone, current resourceRecordSet, rawValue string) provider.DNSRecord {
	content, priority := splitRecordValue(current.Type, rawValue)
	return provider.DNSRecord{
		ID:       encodeRecordID(current.Name, current.Type, rawValue),
		Type:     strings.ToUpper(strings.TrimSpace(current.Type)),
		Name:     normalizeRecordName(current.Name, zoneInfo.DNSName),
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
	priority, err := strconv.Atoi(parts[0])
	if err != nil {
		return strings.TrimSpace(value), nil
	}
	return strings.Join(parts[1:], " "), &priority
}

func findRecordSet(items []resourceRecordSet, name, recordType string) *resourceRecordSet {
	targetName := normalizeComparableName(name)
	targetType := strings.ToUpper(strings.TrimSpace(recordType))
	for index := range items {
		if normalizeComparableName(items[index].Name) == targetName && strings.ToUpper(strings.TrimSpace(items[index].Type)) == targetType {
			return &items[index]
		}
	}
	return nil
}

func findProviderRecord(zoneInfo managedZone, current resourceRecordSet, rawValue string) *provider.DNSRecord {
	for _, item := range current.RRDatas {
		if strings.TrimSpace(item) != strings.TrimSpace(rawValue) {
			continue
		}
		record := mapRecord(zoneInfo, current, item)
		return &record
	}
	return nil
}

func sameRecordSetIdentity(currentName, currentType, desiredName, desiredType, domainName string) bool {
	return normalizeComparableName(currentName) == normalizeComparableName(fqdnRecordName(desiredName, domainName)) && strings.EqualFold(strings.TrimSpace(currentType), strings.TrimSpace(desiredType))
}

func cloneRecordSet(input resourceRecordSet) resourceRecordSet {
	cloned := input
	cloned.RRDatas = cloneStrings(input.RRDatas)
	return cloned
}

func cloneStrings(input []string) []string {
	items := make([]string, len(input))
	copy(items, input)
	return items
}

func encodeRecordID(name, recordType, rawValue string) string {
	parts := []string{
		normalizeComparableName(name),
		strings.ToUpper(strings.TrimSpace(recordType)),
		base64.RawURLEncoding.EncodeToString([]byte(strings.TrimSpace(rawValue))),
	}
	return strings.Join(parts, "|")
}

func decodeRecordID(recordID string) (string, string, string, error) {
	parts := strings.Split(strings.TrimSpace(recordID), "|")
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("invalid gcp record id")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return "", "", "", fmt.Errorf("invalid gcp record id")
	}
	return normalizeComparableName(parts[0]), strings.ToUpper(strings.TrimSpace(parts[1])), strings.TrimSpace(string(decoded)), nil
}

func effectiveTTL(current resourceRecordSet) int {
	return effectiveTTLInt(current.TTL)
}

func effectiveTTLFromSet(current resourceRecordSet, desiredTTL int) int {
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

func normalizePrivateKey(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	trimmed = strings.ReplaceAll(trimmed, "\r\n", "\n")
	trimmed = strings.ReplaceAll(trimmed, "\\n", "\n")
	return trimmed
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
		if short == "" {
			return "@"
		}
		return strings.TrimSuffix(short, ".")
	}
	return trimmedName
}

func normalizeComparableName(name string) string {
	trimmed := strings.TrimSpace(strings.TrimSuffix(name, "."))
	if trimmed == "" {
		return "@"
	}
	return strings.ToLower(trimmed)
}
