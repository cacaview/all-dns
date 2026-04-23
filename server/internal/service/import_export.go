package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"strconv"
	"strings"

	"dns-hub/server/internal/provider"
)

// ExportRecords exports DNS records for a domain in the specified format ("json" or "csv").
func (s *DNSService) ExportRecords(ctx context.Context, userID, domainID uint, format string) ([]byte, string, error) {
	domain, account, err := s.getDomainForUser(domainID, userID)
	if err != nil {
		return nil, "", err
	}
	adapter, err := s.providerForAccount(*account)
	if err != nil {
		return nil, "", err
	}
	records, err := adapter.ListRecords(ctx, domain.ProviderZoneID)
	if err != nil {
		return nil, "", err
	}

	switch strings.ToLower(format) {
	case "csv":
		return exportRecordsCSV(records), fmt.Sprintf("records-%s.csv", domain.Name), nil
	default:
		payload, _ := exportRecordsJSON(records)
		return payload, fmt.Sprintf("records-%s.json", domain.Name), nil
	}
}

func exportRecordsJSON(records []provider.DNSRecord) ([]byte, string) {
	type exportRecord struct {
		Type     string `json:"type"`
		Name     string `json:"name"`
		Content  string `json:"content"`
		TTL      int    `json:"ttl"`
		Priority *int   `json:"priority,omitempty"`
		Proxied  *bool  `json:"proxied,omitempty"`
		Comment  string `json:"comment,omitempty"`
	}
	export := make([]exportRecord, len(records))
	for i, r := range records {
		export[i] = exportRecord{
			Type:     r.Type,
			Name:     r.Name,
			Content:  r.Content,
			TTL:      r.TTL,
			Priority: r.Priority,
			Proxied:  r.Proxied,
			Comment:  r.Comment,
		}
	}
	payload, _ := json.MarshalIndent(export, "", "  ")
	return payload, "application/json"
}

func exportRecordsCSV(records []provider.DNSRecord) []byte {
	buf := &bytes.Buffer{}
	writer := csv.NewWriter(buf)
	// Header
	_ = writer.Write([]string{"Type", "Name", "Content", "TTL", "Priority", "Proxied", "Comment"})
	for _, r := range records {
		priority := ""
		if r.Priority != nil {
			priority = strconv.Itoa(*r.Priority)
		}
		proxied := ""
		if r.Proxied != nil {
			proxied = strconv.FormatBool(*r.Proxied)
		}
		_ = writer.Write([]string{r.Type, r.Name, r.Content, strconv.Itoa(r.TTL), priority, proxied, r.Comment})
	}
	writer.Flush()
	return buf.Bytes()
}

// ImportRecords imports DNS records from a JSON or CSV file.
func (s *DNSService) ImportRecords(ctx context.Context, userID, domainID uint, file multipart.File, filename string) (int, error) {
	domain, account, err := s.getDomainForUser(domainID, userID)
	if err != nil {
		return 0, err
	}
	adapter, err := s.providerForAccount(*account)
	if err != nil {
		return 0, err
	}

	content, err := io.ReadAll(file)
	if err != nil {
		return 0, fmt.Errorf("read file: %w", err)
	}

	var records []provider.RecordMutation
	if strings.HasSuffix(strings.ToLower(filename), ".csv") {
		records, err = parseRecordsCSV(content)
	} else {
		records, err = parseRecordsJSON(content)
	}
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", filename, err)
	}

	imported := 0
	for _, mut := range records {
		if _, err := adapter.UpsertRecord(ctx, domain.ProviderZoneID, mut); err != nil {
			// Continue on error, log the failed record
			continue
		}
		imported++
	}
	return imported, nil
}

func parseRecordsJSON(content []byte) ([]provider.RecordMutation, error) {
	type rawRecord struct {
		Type     string `json:"type"`
		Name     string `json:"name"`
		Content  string `json:"content"`
		TTL      int    `json:"ttl"`
		Priority *int   `json:"priority,omitempty"`
		Proxied  *bool  `json:"proxied,omitempty"`
		Comment  string `json:"comment,omitempty"`
	}
	var raw []rawRecord
	if err := json.Unmarshal(content, &raw); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	records := make([]provider.RecordMutation, 0, len(raw))
	for _, r := range raw {
		if r.Type == "" || r.Name == "" || r.Content == "" {
			continue
		}
		records = append(records, provider.RecordMutation{
			Type:     r.Type,
			Name:     r.Name,
			Content:  r.Content,
			TTL:      r.TTL,
			Priority: r.Priority,
			Proxied:  r.Proxied,
			Comment:  r.Comment,
		})
	}
	return records, nil
}

func parseRecordsCSV(content []byte) ([]provider.RecordMutation, error) {
	reader := csv.NewReader(bytes.NewReader(content))
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("invalid CSV: %w", err)
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("CSV must have a header row and at least one data row")
	}
	// Skip header
	mutations := make([]provider.RecordMutation, 0, len(records)-1)
	for _, row := range records[1:] {
		if len(row) < 4 {
			continue
		}
		ttl := 300
		if row[3] != "" {
			if parsed, err := strconv.Atoi(row[3]); err == nil {
				ttl = parsed
			}
		}
		var priority *int
		if len(row) > 4 && row[4] != "" {
			if p, err := strconv.Atoi(row[4]); err == nil {
				priority = &p
			}
		}
		var proxied *bool
		if len(row) > 5 && row[5] != "" {
			if b, err := strconv.ParseBool(row[5]); err == nil {
				proxied = &b
			}
		}
		comment := ""
		if len(row) > 6 {
			comment = row[6]
		}
		mutations = append(mutations, provider.RecordMutation{
			Type:     row[0],
			Name:     row[1],
			Content:  row[2],
			TTL:     ttl,
			Priority: priority,
			Proxied:  proxied,
			Comment:  comment,
		})
		if mutations[len(mutations)-1].Type == "" || mutations[len(mutations)-1].Name == "" || mutations[len(mutations)-1].Content == "" {
			mutations = mutations[:len(mutations)-1]
		}
	}
	return mutations, nil
}
