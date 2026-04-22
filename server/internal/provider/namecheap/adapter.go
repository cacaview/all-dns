package namecheap

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"dns-hub/server/internal/provider"
	"golang.org/x/net/publicsuffix"
)

const (
	productionURL = "https://api.namecheap.com/xml.response"
	sandboxURL    = "https://api.sandbox.namecheap.com/xml.response"
	pageSize      = 100
)

type Adapter struct {
	apiUser  string
	apiKey   string
	username string
	clientIP string
	sandbox  bool
	client   *http.Client
}

type apiError struct {
	Number  string `xml:"Number,attr"`
	Message string `xml:",chardata"`
}

type domainListResponse struct {
	XMLName         xml.Name                  `xml:"ApiResponse"`
	Status          string                    `xml:"Status,attr"`
	Errors          []apiError                `xml:"Errors>Error"`
	CommandResponse domainListCommandResponse `xml:"CommandResponse"`
}

type domainListCommandResponse struct {
	Result domainListResult `xml:"DomainGetListResult"`
	Paging pagingResult     `xml:"Paging"`
}

type domainListResult struct {
	Domains []domainItem `xml:"Domain"`
}

type domainItem struct {
	ID       string `xml:"ID,attr"`
	Name     string `xml:"Name,attr"`
	IsOurDNS string `xml:"IsOurDNS,attr"`
}

type pagingResult struct {
	TotalItems  int `xml:"TotalItems"`
	CurrentPage int `xml:"CurrentPage"`
	PageSize    int `xml:"PageSize"`
}

type hostsResponse struct {
	XMLName         xml.Name             `xml:"ApiResponse"`
	Status          string               `xml:"Status,attr"`
	Errors          []apiError           `xml:"Errors>Error"`
	CommandResponse hostsCommandResponse `xml:"CommandResponse"`
}

type hostsCommandResponse struct {
	Result hostsResult `xml:"DomainDNSGetHostsResult"`
}

type hostsResult struct {
	Domain        string     `xml:"Domain,attr"`
	IsUsingOurDNS string     `xml:"IsUsingOurDNS,attr"`
	Hosts         []hostItem `xml:"Host"`
}

type hostItem struct {
	HostID  string `xml:"HostId,attr"`
	Name    string `xml:"Name,attr"`
	Type    string `xml:"Type,attr"`
	Address string `xml:"Address,attr"`
	MXPref  string `xml:"MXPref,attr"`
	TTL     string `xml:"TTL,attr"`
}

type baseResponse struct {
	XMLName xml.Name   `xml:"ApiResponse"`
	Status  string     `xml:"Status,attr"`
	Errors  []apiError `xml:"Errors>Error"`
}

func init() {
	provider.MustRegister("namecheap", func(config map[string]any) (provider.DNSProvider, error) {
		return New(config)
	})
	provider.MustRegisterDescriptor(provider.Descriptor{
		Key:         "namecheap",
		Label:       "Namecheap",
		Description: "Namecheap XML DNS API",
		Fields: []provider.FieldSpec{
			{Key: "api_user", Label: "API User", Type: provider.FieldTypeText, Required: true, Placeholder: "your-api-user"},
			{Key: "api_key", Label: "API Key", Type: provider.FieldTypePassword, Required: true, Placeholder: "Namecheap API Key"},
			{Key: "username", Label: "Username", Type: provider.FieldTypeText, Required: false, Placeholder: "默认与 API User 相同", HelpText: "留空时默认使用 API User"},
			{Key: "client_ip", Label: "Client IP", Type: provider.FieldTypeText, Required: true, Placeholder: "203.0.113.10", HelpText: "需要加入 Namecheap API 白名单"},
			{Key: "sandbox", Label: "Use Sandbox", Type: provider.FieldTypeBoolean, Required: false, DefaultValue: false},
		},
		SampleConfig: map[string]any{
			"api_user":  "",
			"api_key":   "",
			"username":  "",
			"client_ip": "127.0.0.1",
			"sandbox":   false,
		},
	})
}

func New(config map[string]any) (*Adapter, error) {
	apiUser, _ := config["api_user"].(string)
	apiKey, _ := config["api_key"].(string)
	username, _ := config["username"].(string)
	clientIP, _ := config["client_ip"].(string)
	sandbox, _ := config["sandbox"].(bool)

	apiUser = strings.TrimSpace(apiUser)
	apiKey = strings.TrimSpace(apiKey)
	username = strings.TrimSpace(username)
	clientIP = strings.TrimSpace(clientIP)
	if username == "" {
		username = apiUser
	}
	if apiUser == "" {
		return nil, fmt.Errorf("namecheap api_user is required")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("namecheap api_key is required")
	}
	if username == "" {
		return nil, fmt.Errorf("namecheap username is required")
	}
	if clientIP == "" {
		return nil, fmt.Errorf("namecheap client_ip is required")
	}

	return &Adapter{
		apiUser:  apiUser,
		apiKey:   apiKey,
		username: username,
		clientIP: clientIP,
		sandbox:  sandbox,
		client:   &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (a *Adapter) Name() string {
	return "namecheap"
}

func (a *Adapter) Validate(ctx context.Context) (*provider.ValidationResult, error) {
	_, err := a.ListDomains(ctx)
	if err != nil {
		return nil, err
	}
	return &provider.ValidationResult{OK: true, Message: "namecheap credentials are valid", CheckedAt: time.Now().UTC()}, nil
}

func (a *Adapter) ListDomains(ctx context.Context) ([]provider.Domain, error) {
	items := make([]provider.Domain, 0)
	page := 1
	for {
		var response domainListResponse
		params := a.baseParams("namecheap.domains.getList")
		params.Set("Page", strconv.Itoa(page))
		params.Set("PageSize", strconv.Itoa(pageSize))
		params.Set("ListType", "ALL")
		params.Set("SortBy", "NAME")
		if err := a.request(ctx, http.MethodGet, params, &response); err != nil {
			return nil, err
		}
		for _, item := range response.CommandResponse.Result.Domains {
			name := strings.TrimSpace(item.Name)
			if name == "" || !truthy(item.IsOurDNS) {
				continue
			}
			items = append(items, provider.Domain{ZoneID: name, Name: name, Provider: a.Name()})
		}
		if len(response.CommandResponse.Result.Domains) == 0 {
			break
		}
		total := response.CommandResponse.Paging.TotalItems
		if total <= 0 || len(items) >= total || len(response.CommandResponse.Result.Domains) < pageSize {
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
	sld, tld, err := splitDomain(domainName)
	if err != nil {
		return nil, err
	}
	var response hostsResponse
	params := a.baseParams("namecheap.domains.dns.getHosts")
	params.Set("SLD", sld)
	params.Set("TLD", tld)
	if err := a.request(ctx, http.MethodGet, params, &response); err != nil {
		return nil, err
	}
	if !truthy(response.CommandResponse.Result.IsUsingOurDNS) {
		return nil, fmt.Errorf("domain %s is not using namecheap dns", domainName)
	}
	items := make([]provider.DNSRecord, 0, len(response.CommandResponse.Result.Hosts))
	for _, item := range response.CommandResponse.Result.Hosts {
		items = append(items, mapRecord(item, domainName))
	}
	return items, nil
}

func (a *Adapter) UpsertRecord(ctx context.Context, zoneID string, input provider.RecordMutation) (*provider.DNSRecord, error) {
	domainName := strings.TrimSpace(zoneID)
	if domainName == "" {
		return nil, fmt.Errorf("domain name is required")
	}
	currentRecords, err := a.ListRecords(ctx, domainName)
	if err != nil {
		return nil, err
	}
	updatedRecords := cloneRecords(currentRecords)
	trimmedID := strings.TrimSpace(input.ID)
	if trimmedID != "" {
		index := findRecordIndex(updatedRecords, trimmedID)
		if index < 0 {
			return nil, fmt.Errorf("record %s not found", trimmedID)
		}
		merged, err := mergeRecord(updatedRecords[index], input, domainName)
		if err != nil {
			return nil, err
		}
		updatedRecords[index] = merged
	} else {
		record, err := mutationToRecord(input, domainName)
		if err != nil {
			return nil, err
		}
		updatedRecords = append(updatedRecords, record)
	}
	if err := a.setHosts(ctx, domainName, updatedRecords); err != nil {
		return nil, err
	}
	latestRecords, err := a.ListRecords(ctx, domainName)
	if err != nil {
		return nil, err
	}
	desired := updatedRecords[len(updatedRecords)-1]
	if trimmedID != "" {
		if index := findRecordIndex(updatedRecords, trimmedID); index >= 0 {
			desired = updatedRecords[index]
		}
	}
	if record := findMatchingRecord(latestRecords, desired); record != nil {
		return record, nil
	}
	desired.ID = ""
	return &desired, nil
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
	currentRecords, err := a.ListRecords(ctx, domainName)
	if err != nil {
		return err
	}
	filtered := make([]provider.DNSRecord, 0, len(currentRecords))
	removed := false
	for _, item := range currentRecords {
		if strings.TrimSpace(item.ID) == trimmedID {
			removed = true
			continue
		}
		filtered = append(filtered, item)
	}
	if !removed {
		return fmt.Errorf("record %s not found", trimmedID)
	}
	return a.setHosts(ctx, domainName, filtered)
}

func (a *Adapter) ExportConfig() map[string]any {
	return map[string]any{
		"api_user":  a.apiUser,
		"api_key":   a.apiKey,
		"username":  a.username,
		"client_ip": a.clientIP,
		"sandbox":   a.sandbox,
	}
}

func (a *Adapter) setHosts(ctx context.Context, domainName string, records []provider.DNSRecord) error {
	sld, tld, err := splitDomain(domainName)
	if err != nil {
		return err
	}
	params := a.baseParams("namecheap.domains.dns.setHosts")
	params.Set("SLD", sld)
	params.Set("TLD", tld)
	for index, item := range records {
		recordIndex := strconv.Itoa(index + 1)
		params.Set("HostName"+recordIndex, normalizeRecordName(item.Name, domainName))
		params.Set("RecordType"+recordIndex, strings.ToUpper(strings.TrimSpace(item.Type)))
		params.Set("Address"+recordIndex, strings.TrimSpace(item.Content))
		if item.TTL > 0 {
			params.Set("TTL"+recordIndex, strconv.Itoa(item.TTL))
		}
		if item.Priority != nil && usesPriority(item.Type) {
			params.Set("MXPref"+recordIndex, strconv.Itoa(*item.Priority))
		}
	}
	var response baseResponse
	return a.request(ctx, http.MethodPost, params, &response)
}

func (a *Adapter) request(ctx context.Context, method string, params url.Values, target any) error {
	endpoint := productionURL
	if a.sandbox {
		endpoint = sandboxURL
	}
	var request *http.Request
	var err error
	if method == http.MethodPost {
		body := params.Encode()
		request, err = http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(body))
		if err != nil {
			return fmt.Errorf("build namecheap request: %w", err)
		}
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		request, err = http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+params.Encode(), nil)
		if err != nil {
			return fmt.Errorf("build namecheap request: %w", err)
		}
	}
	request.Header.Set("Accept", "application/xml, text/xml")

	response, err := a.client.Do(request)
	if err != nil {
		return fmt.Errorf("call namecheap api: %w", err)
	}
	defer response.Body.Close()

	payload, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("read namecheap response: %w", err)
	}
	if response.StatusCode >= 400 {
		return fmt.Errorf("namecheap api returned %s: %s", response.Status, strings.TrimSpace(string(payload)))
	}
	if target == nil || len(payload) == 0 {
		return nil
	}
	if err := xml.Unmarshal(payload, target); err != nil {
		return fmt.Errorf("decode namecheap response: %w", err)
	}
	if err := responseError(target); err != nil {
		return err
	}
	return nil
}

func responseError(target any) error {
	switch typed := target.(type) {
	case *domainListResponse:
		if strings.EqualFold(typed.Status, "OK") {
			return nil
		}
		return fmt.Errorf("namecheap api error: %s", joinErrors(typed.Errors))
	case *hostsResponse:
		if strings.EqualFold(typed.Status, "OK") {
			return nil
		}
		return fmt.Errorf("namecheap api error: %s", joinErrors(typed.Errors))
	case *baseResponse:
		if strings.EqualFold(typed.Status, "OK") {
			return nil
		}
		return fmt.Errorf("namecheap api error: %s", joinErrors(typed.Errors))
	default:
		return nil
	}
}

func joinErrors(items []apiError) string {
	if len(items) == 0 {
		return "unknown error"
	}
	messages := make([]string, 0, len(items))
	for _, item := range items {
		message := strings.TrimSpace(item.Message)
		if number := strings.TrimSpace(item.Number); number != "" && message != "" {
			message = number + ": " + message
		}
		if message != "" {
			messages = append(messages, message)
		}
	}
	if len(messages) == 0 {
		return "unknown error"
	}
	return strings.Join(messages, "; ")
}

func (a *Adapter) baseParams(command string) url.Values {
	return url.Values{
		"ApiUser":  []string{a.apiUser},
		"ApiKey":   []string{a.apiKey},
		"UserName": []string{a.username},
		"Command":  []string{command},
		"ClientIp": []string{a.clientIP},
	}
}

func splitDomain(domainName string) (string, string, error) {
	trimmed := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(domainName)), ".")
	if trimmed == "" {
		return "", "", fmt.Errorf("domain name is required")
	}
	registrable, err := publicsuffix.EffectiveTLDPlusOne(trimmed)
	if err == nil {
		trimmed = registrable
	}
	suffix, _ := publicsuffix.PublicSuffix(trimmed)
	suffix = strings.TrimSpace(suffix)
	if suffix == "" || suffix == trimmed {
		lastDot := strings.LastIndex(trimmed, ".")
		if lastDot <= 0 || lastDot == len(trimmed)-1 {
			return "", "", fmt.Errorf("invalid domain name %s", domainName)
		}
		return trimmed[:lastDot], trimmed[lastDot+1:], nil
	}
	sld := strings.TrimSuffix(trimmed, "."+suffix)
	if sld == "" || strings.Contains(sld, ".") {
		parts := strings.Split(sld, ".")
		sld = parts[len(parts)-1]
	}
	if sld == "" {
		return "", "", fmt.Errorf("invalid domain name %s", domainName)
	}
	return sld, suffix, nil
}

func mapRecord(item hostItem, domainName string) provider.DNSRecord {
	record := provider.DNSRecord{
		ID:      strings.TrimSpace(item.HostID),
		Type:    strings.ToUpper(strings.TrimSpace(item.Type)),
		Name:    normalizeRecordName(item.Name, domainName),
		Content: strings.TrimSpace(item.Address),
		TTL:     parseInt(item.TTL, 60),
	}
	if priority := parseOptionalInt(item.MXPref); priority != nil && usesPriority(record.Type) {
		record.Priority = priority
	}
	return record
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
	name := normalizeRecordName(strings.TrimSpace(input.Name), domainName)
	ttl := input.TTL
	if ttl <= 0 {
		ttl = 60
	}
	return provider.DNSRecord{
		ID:       strings.TrimSpace(input.ID),
		Type:     recordType,
		Name:     name,
		Content:  content,
		TTL:      ttl,
		Priority: input.Priority,
		Proxied:  input.Proxied,
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
	if strings.TrimSpace(updated.Type) == "" {
		return provider.DNSRecord{}, fmt.Errorf("record type is required")
	}
	if strings.TrimSpace(updated.Content) == "" {
		return provider.DNSRecord{}, fmt.Errorf("record content is required")
	}
	return updated, nil
}

func cloneRecords(input []provider.DNSRecord) []provider.DNSRecord {
	items := make([]provider.DNSRecord, len(input))
	copy(items, input)
	return items
}

func findRecordIndex(records []provider.DNSRecord, recordID string) int {
	for index := range records {
		if strings.TrimSpace(records[index].ID) == recordID {
			return index
		}
	}
	return -1
}

func findMatchingRecord(records []provider.DNSRecord, desired provider.DNSRecord) *provider.DNSRecord {
	for index := range records {
		candidate := records[index]
		if strings.TrimSpace(desired.ID) != "" && strings.TrimSpace(candidate.ID) == strings.TrimSpace(desired.ID) {
			return &candidate
		}
		if sameRecord(candidate, desired) {
			return &candidate
		}
	}
	return nil
}

func sameRecord(left, right provider.DNSRecord) bool {
	if strings.ToUpper(strings.TrimSpace(left.Type)) != strings.ToUpper(strings.TrimSpace(right.Type)) {
		return false
	}
	if normalizeComparable(left.Name) != normalizeComparable(right.Name) {
		return false
	}
	if strings.TrimSpace(left.Content) != strings.TrimSpace(right.Content) {
		return false
	}
	if left.TTL != right.TTL {
		return false
	}
	return intPointerValue(left.Priority) == intPointerValue(right.Priority)
}

func normalizeComparable(value string) string {
	trimmed := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(value)), ".")
	if trimmed == "" {
		return "@"
	}
	return trimmed
}

func intPointerValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func parseOptionalInt(value string) *int {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	parsed, err := strconv.Atoi(trimmed)
	if err != nil {
		return nil
	}
	return &parsed
}

func parseInt(value string, fallback int) int {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(trimmed)
	if err != nil {
		return fallback
	}
	return parsed
}

func truthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "yes", "1":
		return true
	default:
		return false
	}
}

func usesPriority(recordType string) bool {
	switch strings.ToUpper(strings.TrimSpace(recordType)) {
	case "MX", "SRV", "CAA":
		return true
	default:
		return false
	}
}

func normalizeRecordName(name, domainName string) string {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" || trimmedName == "@" {
		return "@"
	}
	trimmedDomain := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(domainName)), ".")
	candidate := strings.TrimSuffix(strings.ToLower(trimmedName), ".")
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
	return trimmedName
}
