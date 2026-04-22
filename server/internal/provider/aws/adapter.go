package aws

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"dns-hub/server/internal/provider"
)

const (
	defaultEndpoint = "https://route53.amazonaws.com"
	signingRegion   = "us-east-1"
	serviceName     = "route53"
	apiVersionPath  = "/2013-04-01"
	xmlNamespace    = "https://route53.amazonaws.com/doc/2013-04-01/"
	pageLimit       = 100
	defaultTTL      = 300
)

type Adapter struct {
	accessKeyID     string
	secretAccessKey string
	sessionToken    string
	endpoint        string
	host            string
	client          *http.Client
}

type listHostedZonesResponse struct {
	HostedZones []hostedZone `xml:"HostedZones>HostedZone"`
	IsTruncated bool         `xml:"IsTruncated"`
	NextMarker  string       `xml:"NextMarker"`
}

type getHostedZoneResponse struct {
	HostedZone hostedZone `xml:"HostedZone"`
}

type hostedZone struct {
	ID   string `xml:"Id"`
	Name string `xml:"Name"`
}

type listResourceRecordSetsResponse struct {
	ResourceRecordSets   []resourceRecordSet `xml:"ResourceRecordSets>ResourceRecordSet"`
	IsTruncated          bool                `xml:"IsTruncated"`
	NextRecordName       string              `xml:"NextRecordName"`
	NextRecordType       string              `xml:"NextRecordType"`
	NextRecordIdentifier string              `xml:"NextRecordIdentifier"`
}

type resourceRecordSet struct {
	Name            string           `xml:"Name"`
	Type            string           `xml:"Type"`
	SetIdentifier   string           `xml:"SetIdentifier"`
	TTL             int              `xml:"TTL"`
	ResourceRecords []resourceRecord `xml:"ResourceRecords>ResourceRecord"`
	AliasTarget     *aliasTarget     `xml:"AliasTarget"`
}

type resourceRecord struct {
	Value string `xml:"Value"`
}

type aliasTarget struct {
	HostedZoneID         string `xml:"HostedZoneId"`
	DNSName              string `xml:"DNSName"`
	EvaluateTargetHealth bool   `xml:"EvaluateTargetHealth"`
}

type changeResourceRecordSetsRequest struct {
	XMLName     xml.Name    `xml:"ChangeResourceRecordSetsRequest"`
	Xmlns       string      `xml:"xmlns,attr"`
	ChangeBatch changeBatch `xml:"ChangeBatch"`
}

type changeBatch struct {
	Changes changeList `xml:"Changes"`
}

type changeList struct {
	Items []change `xml:"Change"`
}

type change struct {
	Action            string          `xml:"Action"`
	ResourceRecordSet changeRecordSet `xml:"ResourceRecordSet"`
}

type changeRecordSet struct {
	Name            string                 `xml:"Name"`
	Type            string                 `xml:"Type"`
	SetIdentifier   string                 `xml:"SetIdentifier,omitempty"`
	TTL             *int                   `xml:"TTL,omitempty"`
	ResourceRecords *changeResourceRecords `xml:"ResourceRecords,omitempty"`
	AliasTarget     *aliasTarget           `xml:"AliasTarget,omitempty"`
}

type changeResourceRecords struct {
	Items []resourceRecord `xml:"ResourceRecord"`
}

func init() {
	provider.MustRegister("aws", func(config map[string]any) (provider.DNSProvider, error) {
		return New(config)
	})
	provider.MustRegisterDescriptor(provider.Descriptor{
		Key:         "aws",
		Label:       "AWS Route53",
		Description: "Amazon Route53 API",
		Fields: []provider.FieldSpec{
			{Key: "access_key_id", Label: "Access Key ID", Type: provider.FieldTypeText, Required: true, Placeholder: "AKIA...", HelpText: "AWS IAM Access Key ID"},
			{Key: "secret_access_key", Label: "Secret Access Key", Type: provider.FieldTypePassword, Required: true, Placeholder: "Secret Access Key", HelpText: "AWS IAM Secret Access Key"},
			{Key: "session_token", Label: "Session Token", Type: provider.FieldTypePassword, Required: false, Placeholder: "Session Token", HelpText: "可选，临时凭证时填写"},
			{Key: "endpoint", Label: "API Endpoint", Type: provider.FieldTypeText, Required: false, Placeholder: defaultEndpoint, HelpText: "可选，默认使用 Route53 全局 API Endpoint"},
		},
		SampleConfig: map[string]any{
			"access_key_id":     "",
			"secret_access_key": "",
			"session_token":     "",
			"endpoint":          defaultEndpoint,
		},
	})
}

func New(config map[string]any) (*Adapter, error) {
	accessKeyID, _ := config["access_key_id"].(string)
	secretAccessKey, _ := config["secret_access_key"].(string)
	sessionToken, _ := config["session_token"].(string)
	endpoint, _ := config["endpoint"].(string)
	accessKeyID = strings.TrimSpace(accessKeyID)
	secretAccessKey = strings.TrimSpace(secretAccessKey)
	sessionToken = strings.TrimSpace(sessionToken)
	endpoint = strings.TrimSpace(endpoint)
	if accessKeyID == "" {
		return nil, fmt.Errorf("aws access_key_id is required")
	}
	if secretAccessKey == "" {
		return nil, fmt.Errorf("aws secret_access_key is required")
	}
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	if !strings.Contains(endpoint, "://") {
		endpoint = "https://" + endpoint
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse aws endpoint: %w", err)
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return nil, fmt.Errorf("aws endpoint is invalid")
	}
	return &Adapter{
		accessKeyID:     accessKeyID,
		secretAccessKey: secretAccessKey,
		sessionToken:    sessionToken,
		endpoint:        strings.TrimRight(endpoint, "/"),
		host:            parsed.Host,
		client:          &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (a *Adapter) Name() string {
	return "aws"
}

func (a *Adapter) Validate(ctx context.Context) (*provider.ValidationResult, error) {
	_, err := a.ListDomains(ctx)
	if err != nil {
		return nil, err
	}
	return &provider.ValidationResult{OK: true, Message: "aws credentials are valid", CheckedAt: time.Now().UTC()}, nil
}

func (a *Adapter) ListDomains(ctx context.Context) ([]provider.Domain, error) {
	items := make([]provider.Domain, 0)
	marker := ""
	for {
		query := url.Values{}
		query.Set("maxitems", strconv.Itoa(pageLimit))
		if marker != "" {
			query.Set("marker", marker)
		}
		var response listHostedZonesResponse
		if err := a.request(ctx, http.MethodGet, apiVersionPath+"/hostedzone", query, nil, &response); err != nil {
			return nil, err
		}
		for _, item := range response.HostedZones {
			zoneID := trimHostedZoneID(item.ID)
			name := strings.TrimSuffix(strings.TrimSpace(item.Name), ".")
			if zoneID == "" || name == "" {
				continue
			}
			items = append(items, provider.Domain{ZoneID: zoneID, Name: name, Provider: a.Name()})
		}
		if !response.IsTruncated || strings.TrimSpace(response.NextMarker) == "" {
			break
		}
		marker = strings.TrimSpace(response.NextMarker)
	}
	return items, nil
}

func (a *Adapter) ListRecords(ctx context.Context, zoneID string) ([]provider.DNSRecord, error) {
	zoneInfo, err := a.getHostedZone(ctx, zoneID)
	if err != nil {
		return nil, err
	}
	recordsets, err := a.listRecordSets(ctx, zoneInfo.ID)
	if err != nil {
		return nil, err
	}
	items := make([]provider.DNSRecord, 0)
	for _, item := range recordsets {
		if item.AliasTarget != nil {
			items = append(items, mapAliasRecord(zoneInfo, item))
			continue
		}
		for _, value := range item.ResourceRecords {
			items = append(items, mapRecord(zoneInfo, item, value.Value))
		}
	}
	return items, nil
}

func (a *Adapter) UpsertRecord(ctx context.Context, zoneID string, input provider.RecordMutation) (*provider.DNSRecord, error) {
	zoneInfo, err := a.getHostedZone(ctx, zoneID)
	if err != nil {
		return nil, err
	}
	recordsets, err := a.listRecordSets(ctx, zoneInfo.ID)
	if err != nil {
		return nil, err
	}
	trimmedID := strings.TrimSpace(input.ID)
	if trimmedID == "" {
		desired, err := mutationToRecord(input, zoneInfo.Name)
		if err != nil {
			return nil, err
		}
		return a.upsertDesiredRecord(ctx, zoneInfo, recordsets, desired)
	}
	name, recordType, setIdentifier, rawValue, err := decodeRecordID(trimmedID)
	if err != nil {
		return nil, err
	}
	current := findRecordSet(recordsets, name, recordType, setIdentifier)
	if current == nil {
		return nil, fmt.Errorf("record %s not found", trimmedID)
	}
	if current.AliasTarget != nil {
		return nil, fmt.Errorf("route53 alias records are not editable in this adapter")
	}
	currentRecord := findProviderRecord(zoneInfo, *current, rawValue)
	if currentRecord == nil {
		return nil, fmt.Errorf("record %s not found", trimmedID)
	}
	desired, err := mergeRecord(*currentRecord, input, zoneInfo.Name)
	if err != nil {
		return nil, err
	}
	if sameRecordSetIdentity(current.Name, current.Type, desired.Name, desired.Type, zoneInfo.Name) {
		if err := a.replaceRecordInSet(ctx, zoneInfo.ID, *current, rawValue, desired); err != nil {
			return nil, err
		}
		return a.readBackRecord(ctx, zoneInfo, desired, setIdentifier)
	}
	if err := a.removeRecordFromSet(ctx, zoneInfo.ID, *current, rawValue); err != nil {
		return nil, err
	}
	updatedSets, err := a.listRecordSets(ctx, zoneInfo.ID)
	if err != nil {
		return nil, err
	}
	return a.upsertDesiredRecord(ctx, zoneInfo, updatedSets, desired)
}

func (a *Adapter) DeleteRecord(ctx context.Context, zoneID string, recordID string) error {
	zoneInfo, err := a.getHostedZone(ctx, zoneID)
	if err != nil {
		return err
	}
	targetName, recordType, setIdentifier, rawValue, err := decodeRecordID(recordID)
	if err != nil {
		return err
	}
	recordsets, err := a.listRecordSets(ctx, zoneInfo.ID)
	if err != nil {
		return err
	}
	current := findRecordSet(recordsets, targetName, recordType, setIdentifier)
	if current == nil {
		return fmt.Errorf("record %s not found", strings.TrimSpace(recordID))
	}
	if current.AliasTarget != nil {
		return a.submitChange(ctx, zoneInfo.ID, "DELETE", *current)
	}
	return a.removeRecordFromSet(ctx, zoneInfo.ID, *current, rawValue)
}

func (a *Adapter) ExportConfig() map[string]any {
	return map[string]any{
		"access_key_id":     a.accessKeyID,
		"secret_access_key": a.secretAccessKey,
		"session_token":     a.sessionToken,
		"endpoint":          a.endpoint,
	}
}

func (a *Adapter) getHostedZone(ctx context.Context, zoneID string) (hostedZone, error) {
	trimmed := trimHostedZoneID(zoneID)
	if trimmed == "" {
		return hostedZone{}, fmt.Errorf("zone id is required")
	}
	var response getHostedZoneResponse
	if err := a.request(ctx, http.MethodGet, apiVersionPath+"/hostedzone/"+url.PathEscape(trimmed), nil, nil, &response); err != nil {
		return hostedZone{}, err
	}
	response.HostedZone.ID = trimHostedZoneID(response.HostedZone.ID)
	if response.HostedZone.ID == "" {
		return hostedZone{}, fmt.Errorf("zone %s not found", trimmed)
	}
	return response.HostedZone, nil
}

func (a *Adapter) listRecordSets(ctx context.Context, zoneID string) ([]resourceRecordSet, error) {
	trimmed := trimHostedZoneID(zoneID)
	if trimmed == "" {
		return nil, fmt.Errorf("zone id is required")
	}
	items := make([]resourceRecordSet, 0)
	startName := ""
	startType := ""
	startIdentifier := ""
	for {
		query := url.Values{}
		query.Set("maxitems", strconv.Itoa(pageLimit))
		if startName != "" {
			query.Set("name", startName)
		}
		if startType != "" {
			query.Set("type", startType)
		}
		if startIdentifier != "" {
			query.Set("identifier", startIdentifier)
		}
		var response listResourceRecordSetsResponse
		path := apiVersionPath + "/hostedzone/" + url.PathEscape(trimmed) + "/rrset"
		if err := a.request(ctx, http.MethodGet, path, query, nil, &response); err != nil {
			return nil, err
		}
		items = append(items, response.ResourceRecordSets...)
		if !response.IsTruncated || strings.TrimSpace(response.NextRecordName) == "" {
			break
		}
		startName = strings.TrimSpace(response.NextRecordName)
		startType = strings.TrimSpace(response.NextRecordType)
		startIdentifier = strings.TrimSpace(response.NextRecordIdentifier)
	}
	return items, nil
}

func (a *Adapter) upsertDesiredRecord(ctx context.Context, zoneInfo hostedZone, recordsets []resourceRecordSet, desired provider.DNSRecord) (*provider.DNSRecord, error) {
	recordsetName := fqdnRecordName(desired.Name, zoneInfo.Name)
	recordType := strings.ToUpper(strings.TrimSpace(desired.Type))
	rawValue, err := buildRecordValue(recordType, desired.Content, desired.Priority)
	if err != nil {
		return nil, err
	}
	current := findRecordSet(recordsets, recordsetName, recordType, "")
	if current == nil {
		newSet := resourceRecordSet{
			Name:            recordsetName,
			Type:            recordType,
			TTL:             effectiveTTLInt(desired.TTL),
			ResourceRecords: []resourceRecord{{Value: rawValue}},
		}
		if err := a.submitChange(ctx, zoneInfo.ID, "CREATE", newSet); err != nil {
			return nil, err
		}
		return a.readBackRecord(ctx, zoneInfo, desired, "")
	}
	if current.AliasTarget != nil {
		return nil, fmt.Errorf("route53 alias record set %s cannot be merged with standard records", recordsetName)
	}
	updated := cloneRecordSet(*current)
	updated.TTL = effectiveTTLFromSet(*current, desired.TTL)
	found := false
	for _, item := range updated.ResourceRecords {
		if strings.TrimSpace(item.Value) == strings.TrimSpace(rawValue) {
			found = true
			break
		}
	}
	if !found {
		updated.ResourceRecords = append(updated.ResourceRecords, resourceRecord{Value: rawValue})
	}
	if err := a.submitChange(ctx, zoneInfo.ID, "UPSERT", updated); err != nil {
		return nil, err
	}
	return a.readBackRecord(ctx, zoneInfo, desired, current.SetIdentifier)
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
	for index := range updated.ResourceRecords {
		if strings.TrimSpace(updated.ResourceRecords[index].Value) != strings.TrimSpace(oldValue) {
			continue
		}
		updated.ResourceRecords[index].Value = newValue
		replaced = true
		break
	}
	if !replaced {
		return fmt.Errorf("record %s not found", encodeRecordID(current.Name, current.Type, current.SetIdentifier, oldValue))
	}
	return a.submitChange(ctx, zoneID, "UPSERT", updated)
}

func (a *Adapter) removeRecordFromSet(ctx context.Context, zoneID string, current resourceRecordSet, rawValue string) error {
	if current.AliasTarget != nil {
		return a.submitChange(ctx, zoneID, "DELETE", current)
	}
	filtered := make([]resourceRecord, 0, len(current.ResourceRecords))
	removed := false
	for _, item := range current.ResourceRecords {
		if strings.TrimSpace(item.Value) == strings.TrimSpace(rawValue) {
			removed = true
			continue
		}
		filtered = append(filtered, item)
	}
	if !removed {
		return fmt.Errorf("record %s not found", encodeRecordID(current.Name, current.Type, current.SetIdentifier, rawValue))
	}
	if len(filtered) == 0 {
		return a.submitChange(ctx, zoneID, "DELETE", current)
	}
	updated := cloneRecordSet(current)
	updated.ResourceRecords = filtered
	return a.submitChange(ctx, zoneID, "UPSERT", updated)
}

func (a *Adapter) readBackRecord(ctx context.Context, zoneInfo hostedZone, desired provider.DNSRecord, setIdentifier string) (*provider.DNSRecord, error) {
	recordsets, err := a.listRecordSets(ctx, zoneInfo.ID)
	if err != nil {
		return nil, err
	}
	targetName := normalizeComparableName(fqdnRecordName(desired.Name, zoneInfo.Name))
	targetType := strings.ToUpper(strings.TrimSpace(desired.Type))
	targetValue, err := buildRecordValue(desired.Type, desired.Content, desired.Priority)
	if err != nil {
		return nil, err
	}
	for _, current := range recordsets {
		if normalizeComparableName(current.Name) != targetName || strings.ToUpper(strings.TrimSpace(current.Type)) != targetType || strings.TrimSpace(current.SetIdentifier) != strings.TrimSpace(setIdentifier) {
			continue
		}
		for _, item := range current.ResourceRecords {
			if strings.TrimSpace(item.Value) != strings.TrimSpace(targetValue) {
				continue
			}
			record := mapRecord(zoneInfo, current, item.Value)
			return &record, nil
		}
	}
	return nil, fmt.Errorf("record not found after route53 change")
}

func (a *Adapter) submitChange(ctx context.Context, zoneID, action string, recordset resourceRecordSet) error {
	requestBody := changeResourceRecordSetsRequest{
		Xmlns: xmlNamespace,
		ChangeBatch: changeBatch{
			Changes: changeList{
				Items: []change{{
					Action:            strings.ToUpper(strings.TrimSpace(action)),
					ResourceRecordSet: toChangeRecordSet(recordset),
				}},
			},
		},
	}
	payload, err := xml.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("marshal route53 change request: %w", err)
	}
	path := apiVersionPath + "/hostedzone/" + url.PathEscape(trimHostedZoneID(zoneID)) + "/rrset"
	return a.request(ctx, http.MethodPost, path, nil, payload, nil)
}

func toChangeRecordSet(recordset resourceRecordSet) changeRecordSet {
	changeSet := changeRecordSet{
		Name:          fqdnRecordName(recordset.Name, ""),
		Type:          strings.ToUpper(strings.TrimSpace(recordset.Type)),
		SetIdentifier: strings.TrimSpace(recordset.SetIdentifier),
		AliasTarget:   recordset.AliasTarget,
	}
	if recordset.AliasTarget == nil {
		ttl := effectiveTTL(recordset)
		changeSet.TTL = &ttl
		changeSet.ResourceRecords = &changeResourceRecords{Items: cloneResourceRecords(recordset.ResourceRecords)}
	}
	return changeSet
}

func (a *Adapter) request(ctx context.Context, method, path string, query url.Values, body []byte, target any) error {
	requestURL := a.endpoint + path
	if len(query) > 0 {
		requestURL += "?" + query.Encode()
	}
	var reader io.Reader
	payload := body
	if payload == nil {
		payload = []byte{}
	}
	if len(payload) > 0 {
		reader = bytes.NewReader(payload)
	}
	request, err := http.NewRequestWithContext(ctx, method, requestURL, reader)
	if err != nil {
		return fmt.Errorf("build route53 request: %w", err)
	}
	amzDate := time.Now().UTC().Format("20060102T150405Z")
	contentHash := sha256Hex(payload)
	request.Header.Set("Host", a.host)
	request.Header.Set("X-Amz-Date", amzDate)
	request.Header.Set("X-Amz-Content-Sha256", contentHash)
	request.Header.Set("Accept", "application/xml")
	if len(payload) > 0 {
		request.Header.Set("Content-Type", "application/xml")
	}
	if a.sessionToken != "" {
		request.Header.Set("X-Amz-Security-Token", a.sessionToken)
	}
	request.Header.Set("Authorization", a.authorization(request, contentHash, amzDate))
	response, err := a.client.Do(request)
	if err != nil {
		return fmt.Errorf("call route53 api: %w", err)
	}
	defer response.Body.Close()
	responsePayload, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("read route53 response: %w", err)
	}
	if response.StatusCode >= 400 {
		return fmt.Errorf("route53 api returned %s: %s", response.Status, strings.TrimSpace(string(responsePayload)))
	}
	if target == nil || len(responsePayload) == 0 {
		return nil
	}
	if err := xml.Unmarshal(responsePayload, target); err != nil {
		return fmt.Errorf("decode route53 response: %w", err)
	}
	return nil
}

func (a *Adapter) authorization(request *http.Request, payloadHash string, amzDate string) string {
	canonicalURI := request.URL.EscapedPath()
	if canonicalURI == "" {
		canonicalURI = "/"
	}
	canonicalQuery := canonicalQueryString(request.URL.Query())
	headers := map[string]string{
		"host":                 a.host,
		"x-amz-content-sha256": payloadHash,
		"x-amz-date":           amzDate,
	}
	if a.sessionToken != "" {
		headers["x-amz-security-token"] = a.sessionToken
	}
	headerKeys := make([]string, 0, len(headers))
	for key := range headers {
		headerKeys = append(headerKeys, key)
	}
	sort.Strings(headerKeys)
	canonicalHeaders := strings.Builder{}
	for _, key := range headerKeys {
		canonicalHeaders.WriteString(key)
		canonicalHeaders.WriteString(":")
		canonicalHeaders.WriteString(strings.TrimSpace(headers[key]))
		canonicalHeaders.WriteString("\n")
	}
	signedHeaders := strings.Join(headerKeys, ";")
	canonicalRequest := strings.Join([]string{
		request.Method,
		canonicalURI,
		canonicalQuery,
		canonicalHeaders.String(),
		signedHeaders,
		payloadHash,
	}, "\n")
	date := amzDate[:8]
	credentialScope := date + "/" + signingRegion + "/" + serviceName + "/aws4_request"
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		credentialScope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")
	secretDate := hmacSHA256([]byte("AWS4"+a.secretAccessKey), date)
	secretRegion := hmacSHA256(secretDate, signingRegion)
	secretService := hmacSHA256(secretRegion, serviceName)
	signingKey := hmacSHA256(secretService, "aws4_request")
	signature := hex.EncodeToString(hmacSHA256(signingKey, stringToSign))
	return fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s", a.accessKeyID, credentialScope, signedHeaders, signature)
}

func canonicalQueryString(values url.Values) string {
	if len(values) == 0 {
		return ""
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0)
	for _, key := range keys {
		items := append([]string(nil), values[key]...)
		sort.Strings(items)
		for _, item := range items {
			parts = append(parts, awsPercentEncode(key)+"="+awsPercentEncode(item))
		}
	}
	return strings.Join(parts, "&")
}

func awsPercentEncode(value string) string {
	escaped := url.QueryEscape(value)
	escaped = strings.ReplaceAll(escaped, "+", "%20")
	escaped = strings.ReplaceAll(escaped, "*", "%2A")
	escaped = strings.ReplaceAll(escaped, "%7E", "~")
	return escaped
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

func mapRecord(zoneInfo hostedZone, current resourceRecordSet, rawValue string) provider.DNSRecord {
	content, priority := splitRecordValue(current.Type, rawValue)
	return provider.DNSRecord{
		ID:       encodeRecordID(current.Name, current.Type, current.SetIdentifier, rawValue),
		Type:     strings.ToUpper(strings.TrimSpace(current.Type)),
		Name:     normalizeRecordName(current.Name, zoneInfo.Name),
		Content:  content,
		TTL:      effectiveTTL(current),
		Priority: priority,
	}
}

func mapAliasRecord(zoneInfo hostedZone, current resourceRecordSet) provider.DNSRecord {
	content := ""
	if current.AliasTarget != nil {
		content = strings.TrimSuffix(strings.TrimSpace(current.AliasTarget.DNSName), ".")
	}
	return provider.DNSRecord{
		ID:      encodeRecordID(current.Name, current.Type, current.SetIdentifier, "alias:"+content),
		Type:    strings.ToUpper(strings.TrimSpace(current.Type)),
		Name:    normalizeRecordName(current.Name, zoneInfo.Name),
		Content: content,
		TTL:     effectiveTTL(current),
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
	trimmedType := strings.ToUpper(strings.TrimSpace(recordType))
	trimmedValue := strings.TrimSpace(value)
	if trimmedType != "MX" {
		if strings.HasPrefix(strings.ToLower(trimmedValue), "alias:") {
			return strings.TrimSpace(strings.TrimPrefix(trimmedValue, "alias:")), nil
		}
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
	return strings.Join(parts[1:], " "), &priority
}

func findRecordSet(items []resourceRecordSet, name, recordType, setIdentifier string) *resourceRecordSet {
	targetName := normalizeComparableName(name)
	targetType := strings.ToUpper(strings.TrimSpace(recordType))
	targetSetIdentifier := strings.TrimSpace(setIdentifier)
	for index := range items {
		if normalizeComparableName(items[index].Name) == targetName && strings.ToUpper(strings.TrimSpace(items[index].Type)) == targetType && strings.TrimSpace(items[index].SetIdentifier) == targetSetIdentifier {
			return &items[index]
		}
	}
	return nil
}

func findProviderRecord(zoneInfo hostedZone, current resourceRecordSet, rawValue string) *provider.DNSRecord {
	for _, item := range current.ResourceRecords {
		if strings.TrimSpace(item.Value) != strings.TrimSpace(rawValue) {
			continue
		}
		record := mapRecord(zoneInfo, current, item.Value)
		return &record
	}
	return nil
}

func sameRecordSetIdentity(currentName, currentType, desiredName, desiredType, domainName string) bool {
	return normalizeComparableName(currentName) == normalizeComparableName(fqdnRecordName(desiredName, domainName)) && strings.EqualFold(strings.TrimSpace(currentType), strings.TrimSpace(desiredType))
}

func cloneRecordSet(input resourceRecordSet) resourceRecordSet {
	cloned := input
	cloned.ResourceRecords = cloneResourceRecords(input.ResourceRecords)
	if input.AliasTarget != nil {
		alias := *input.AliasTarget
		cloned.AliasTarget = &alias
	}
	return cloned
}

func cloneResourceRecords(input []resourceRecord) []resourceRecord {
	items := make([]resourceRecord, len(input))
	copy(items, input)
	return items
}

func encodeRecordID(name, recordType, setIdentifier, rawValue string) string {
	parts := []string{
		normalizeComparableName(name),
		strings.ToUpper(strings.TrimSpace(recordType)),
		base64.RawURLEncoding.EncodeToString([]byte(strings.TrimSpace(setIdentifier))),
		base64.RawURLEncoding.EncodeToString([]byte(strings.TrimSpace(rawValue))),
	}
	return strings.Join(parts, "|")
}

func decodeRecordID(recordID string) (string, string, string, string, error) {
	parts := strings.Split(strings.TrimSpace(recordID), "|")
	if len(parts) != 4 {
		return "", "", "", "", fmt.Errorf("invalid aws record id")
	}
	setIdentifier, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return "", "", "", "", fmt.Errorf("invalid aws record id")
	}
	rawValue, err := base64.RawURLEncoding.DecodeString(parts[3])
	if err != nil {
		return "", "", "", "", fmt.Errorf("invalid aws record id")
	}
	return normalizeComparableName(parts[0]), strings.ToUpper(strings.TrimSpace(parts[1])), strings.TrimSpace(string(setIdentifier)), strings.TrimSpace(string(rawValue)), nil
}

func trimHostedZoneID(value string) string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.TrimPrefix(trimmed, "/hostedzone/")
	trimmed = strings.TrimPrefix(trimmed, "hostedzone/")
	return strings.TrimSpace(trimmed)
}

func effectiveTTL(current resourceRecordSet) int {
	if current.TTL > 0 {
		return current.TTL
	}
	return defaultTTL
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

func fqdnRecordName(name, domainName string) string {
	trimmedName := strings.TrimSuffix(strings.TrimSpace(name), ".")
	trimmedDomain := strings.TrimSuffix(strings.TrimSpace(domainName), ".")
	if trimmedName == "" || trimmedName == "@" {
		if trimmedDomain == "" {
			return "@"
		}
		return trimmedDomain + "."
	}
	if trimmedDomain == "" {
		return trimmedName + "."
	}
	if strings.EqualFold(trimmedName, trimmedDomain) {
		return trimmedDomain + "."
	}
	suffix := "." + strings.ToLower(trimmedDomain)
	if strings.HasSuffix(strings.ToLower(trimmedName), suffix) {
		return strings.TrimSuffix(trimmedName, ".") + "."
	}
	return trimmedName + "." + trimmedDomain + "."
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
