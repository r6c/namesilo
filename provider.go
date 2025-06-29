// Package namesilo implements a DNS record management client compatible
// with the libdns interfaces for NameSilo.
package namesilo

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

	"github.com/libdns/libdns"
)

const (
	apiEndpoint = "https://www.namesilo.com/api/"
	minTTL      = 300  // Minimum TTL in seconds (5 minutes)
	defaultTTL  = 3600 // Default TTL in seconds (1 hour)
)

// Provider facilitates DNS record manipulation with NameSilo.
type Provider struct {
	APIToken string `json:"api_token,omitempty"`
}

// apiResponse represents the common response structure from NameSilo API
type apiResponse struct {
	Code   int    `xml:"reply>code"`
	Detail string `xml:"reply>detail"`
}

// dnsListResponse represents the response from dnsListRecords
type dnsListResponse struct {
	apiResponse
	Records []dnsRecord `xml:"reply>resource_record"`
}

// dnsRecord represents a DNS record from NameSilo API
type dnsRecord struct {
	ID       string `xml:"record_id"`
	Type     string `xml:"type"`
	Host     string `xml:"host"`
	Value    string `xml:"value"`
	TTL      int    `xml:"ttl"`
	Distance int    `xml:"distance"`
}

// dnsAddResponse represents the response from dnsAddRecord
type dnsAddResponse struct {
	apiResponse
	RecordID string `xml:"reply>record_id"`
}

// dnsUpdateResponse represents the response from dnsUpdateRecord
type dnsUpdateResponse struct {
	apiResponse
	RecordID string `xml:"reply>record_id"`
}

// buildAPIURL constructs a properly encoded API URL
func (p *Provider) buildAPIURL(operation string, params map[string]string) (string, error) {
	u, err := url.Parse(apiEndpoint + operation)
	if err != nil {
		return "", fmt.Errorf("failed to parse API endpoint: %w", err)
	}

	q := u.Query()

	// Add standard parameters
	q.Set("version", "1")
	q.Set("type", "xml")
	q.Set("key", p.APIToken)

	// Add custom parameters
	for key, value := range params {
		if value != "" {
			q.Set(key, value)
		}
	}

	u.RawQuery = q.Encode()
	return u.String(), nil
}

// normalizeRecordName converts a record name relative to the zone
func normalizeRecordName(name, zone string) string {
	zone = strings.TrimSuffix(zone, ".")

	// Handle root record
	if name == "@" || name == "" || name == zone {
		return "@"
	}

	// Handle already absolute names
	if strings.HasSuffix(name, "."+zone) {
		return strings.TrimSuffix(name, "."+zone)
	}

	// Return as-is for relative names
	return name
}

// validateTTL ensures TTL is within acceptable range
func validateTTL(ttl time.Duration) int {
	seconds := int(ttl.Seconds())
	if seconds < minTTL {
		return defaultTTL
	}
	return seconds
}

// extractRecordData extracts specific record data based on type
func extractRecordData(rec libdns.Record) (string, int) {
	var priority int
	var value string

	switch r := rec.(type) {
	case libdns.MX:
		priority = int(r.Preference)
		value = r.Target
	case libdns.SRV:
		priority = int(r.Priority)
		value = fmt.Sprintf("%d %d %s", r.Weight, r.Port, r.Target)
	default:
		// For most record types, get the data from RR()
		rr := rec.RR()
		value = rr.Data
	}

	return value, priority
}

// namesileoRecord wraps libdns records with NameSilo-specific data
type namesileoRecord struct {
	libdns.Record
	ID string // NameSilo record ID
}

// RR implements libdns.Record interface
func (r namesileoRecord) RR() libdns.RR {
	return r.Record.RR()
}

// createLibDNSRecord creates appropriate libdns.Record from NameSilo response
func createLibDNSRecord(nsRecord dnsRecord) libdns.Record {
	var baseRecord libdns.Record

	switch strings.ToUpper(nsRecord.Type) {
	case "A", "AAAA":
		baseRecord = libdns.RR{
			Name: nsRecord.Host,
			Type: nsRecord.Type,
			Data: nsRecord.Value,
			TTL:  time.Duration(nsRecord.TTL) * time.Second,
		}
	case "MX":
		baseRecord = libdns.MX{
			Name:       nsRecord.Host,
			TTL:        time.Duration(nsRecord.TTL) * time.Second,
			Preference: uint16(nsRecord.Distance),
			Target:     nsRecord.Value,
		}
	case "TXT":
		baseRecord = libdns.TXT{
			Name: nsRecord.Host,
			TTL:  time.Duration(nsRecord.TTL) * time.Second,
			Text: nsRecord.Value,
		}
	case "CNAME":
		baseRecord = libdns.CNAME{
			Name:   nsRecord.Host,
			TTL:    time.Duration(nsRecord.TTL) * time.Second,
			Target: nsRecord.Value,
		}
	case "NS":
		baseRecord = libdns.NS{
			Name:   nsRecord.Host,
			TTL:    time.Duration(nsRecord.TTL) * time.Second,
			Target: nsRecord.Value,
		}
	case "SRV":
		// Parse SRV data: "weight port target"
		parts := strings.Fields(nsRecord.Value)
		if len(parts) >= 3 {
			weight, err := strconv.ParseUint(parts[0], 10, 16)
			if err != nil {
				weight = 0 // Default weight if parsing fails
			}
			port, err := strconv.ParseUint(parts[1], 10, 16)
			if err != nil {
				// If port parsing fails, fall back to generic RR
				baseRecord = libdns.RR{
					Name: nsRecord.Host,
					Type: nsRecord.Type,
					Data: nsRecord.Value,
					TTL:  time.Duration(nsRecord.TTL) * time.Second,
				}
			} else {
				target := strings.Join(parts[2:], " ")
				baseRecord = libdns.SRV{
					Name:     nsRecord.Host,
					TTL:      time.Duration(nsRecord.TTL) * time.Second,
					Priority: uint16(nsRecord.Distance),
					Weight:   uint16(weight),
					Port:     uint16(port),
					Target:   target,
				}
			}
		} else {
			baseRecord = libdns.RR{
				Name: nsRecord.Host,
				Type: nsRecord.Type,
				Data: nsRecord.Value,
				TTL:  time.Duration(nsRecord.TTL) * time.Second,
			}
		}
	default:
		// Generic RR for unsupported types
		baseRecord = libdns.RR{
			Name: nsRecord.Host,
			Type: nsRecord.Type,
			Data: nsRecord.Value,
			TTL:  time.Duration(nsRecord.TTL) * time.Second,
		}
	}

	// Wrap with NameSilo-specific data
	return namesileoRecord{
		Record: baseRecord,
		ID:     nsRecord.ID,
	}
}

// GetRecords lists all the records in the zone.
func (p *Provider) GetRecords(ctx context.Context, zone string) ([]libdns.Record, error) {
	if p.APIToken == "" {
		return nil, fmt.Errorf("API token is required")
	}

	domain := strings.TrimSuffix(zone, ".")
	params := map[string]string{
		"domain": domain,
	}

	apiURL, err := p.buildAPIURL("dnsListRecords", params)
	if err != nil {
		return nil, fmt.Errorf("failed to build API URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	var response dnsListResponse
	if err := p.doHTTPRequest(client, req, &response); err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if response.Code != 300 {
		return nil, fmt.Errorf("API error for zone %q: code %d - %s", zone, response.Code, response.Detail)
	}

	var records []libdns.Record
	for _, record := range response.Records {
		rec := createLibDNSRecord(record)
		records = append(records, rec)
	}

	return records, nil
}

// AppendRecords adds records to the zone. It returns the records that were added.
func (p *Provider) AppendRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	if p.APIToken == "" {
		return nil, fmt.Errorf("API token is required")
	}

	domain := strings.TrimSuffix(zone, ".")
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	var appendedRecords []libdns.Record

	for _, record := range records {
		rr := record.RR()
		normalizedName := normalizeRecordName(rr.Name, zone)
		ttl := validateTTL(rr.TTL)
		value, priority := extractRecordData(record)

		params := map[string]string{
			"domain":  domain,
			"rrtype":  rr.Type,
			"rrhost":  normalizedName,
			"rrvalue": value,
			"rrttl":   fmt.Sprintf("%d", ttl),
		}

		// Add distance/priority for MX/SRV records
		if priority > 0 {
			params["rrdistance"] = fmt.Sprintf("%d", priority)
		}

		apiURL, err := p.buildAPIURL("dnsAddRecord", params)
		if err != nil {
			return appendedRecords, fmt.Errorf("failed to build API URL: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
		if err != nil {
			return appendedRecords, fmt.Errorf("failed to create request: %w", err)
		}

		var response dnsAddResponse
		if err := p.doHTTPRequest(client, req, &response); err != nil {
			return appendedRecords, fmt.Errorf("request failed: %w", err)
		}

		if response.Code != 300 {
			return appendedRecords, fmt.Errorf("failed to add record for zone %q: code %d - %s", zone, response.Code, response.Detail)
		}

		// Return the same record type that was passed in
		appendedRecords = append(appendedRecords, record)
	}

	return appendedRecords, nil
}

// SetRecords sets the records in the zone, either by updating existing records or creating new ones.
// It returns the updated records.
func (p *Provider) SetRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	if p.APIToken == "" {
		return nil, fmt.Errorf("API token is required")
	}

	existingRecords, err := p.GetRecords(ctx, zone)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve existing records: %w", err)
	}

	// Create map of existing records by name+type for lookup
	existingMap := make(map[string]libdns.Record)
	for _, rec := range existingRecords {
		rr := rec.RR()
		key := rr.Name + ":" + rr.Type
		existingMap[key] = rec
	}

	var resultRecords []libdns.Record

	// For each input record, either update existing or create new
	for _, record := range records {
		rr := record.RR()
		key := rr.Name + ":" + rr.Type

		if _, exists := existingMap[key]; exists {
			// Update existing record via delete + add
			// First delete the existing record
			if err := p.deleteRecordByNameType(ctx, zone, rr.Name, rr.Type); err != nil {
				return resultRecords, fmt.Errorf("failed to delete existing record: %w", err)
			}
		}

		// Add the new record
		addedRecords, err := p.AppendRecords(ctx, zone, []libdns.Record{record})
		if err != nil {
			return resultRecords, fmt.Errorf("failed to add record: %w", err)
		}

		resultRecords = append(resultRecords, addedRecords...)
	}

	return resultRecords, nil
}

// DeleteRecords deletes the records from the zone. It returns the records that were deleted.
func (p *Provider) DeleteRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	if p.APIToken == "" {
		return nil, fmt.Errorf("API token is required")
	}

	// Get existing records to find IDs
	existingRecords, err := p.GetRecords(ctx, zone)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve existing records: %w", err)
	}

	var deletedRecords []libdns.Record

	for _, record := range records {
		rr := record.RR()
		recordID := p.findRecordID(existingRecords, rr.Name, rr.Type, rr.Data)

		if recordID == "" {
			// Record not found, skip silently as per libdns spec
			continue
		}

		if err := p.deleteRecordByID(ctx, zone, recordID); err != nil {
			return deletedRecords, fmt.Errorf("failed to delete record: %w", err)
		}

		deletedRecords = append(deletedRecords, record)
	}

	return deletedRecords, nil
}

// Helper method to delete a record by name and type
func (p *Provider) deleteRecordByNameType(ctx context.Context, zone, name, recordType string) error {
	existingRecords, err := p.GetRecords(ctx, zone)
	if err != nil {
		return err
	}

	recordID := p.findRecordIDByNameType(existingRecords, name, recordType)
	if recordID == "" {
		return fmt.Errorf("record not found: %s %s", name, recordType)
	}

	return p.deleteRecordByID(ctx, zone, recordID)
}

// Helper method to delete a record by ID
func (p *Provider) deleteRecordByID(ctx context.Context, zone, recordID string) error {
	domain := strings.TrimSuffix(zone, ".")
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	params := map[string]string{
		"domain": domain,
		"rrid":   recordID,
	}

	apiURL, err := p.buildAPIURL("dnsDeleteRecord", params)
	if err != nil {
		return fmt.Errorf("failed to build API URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	var response apiResponse
	if err := p.doHTTPRequest(client, req, &response); err != nil {
		return fmt.Errorf("delete request failed: %w", err)
	}

	if response.Code != 300 {
		return fmt.Errorf("failed to delete record for zone %q: code %d - %s", zone, response.Code, response.Detail)
	}

	return nil
}

// Helper method to find record ID by exact match
func (p *Provider) findRecordID(records []libdns.Record, name, recordType, data string) string {
	for _, rec := range records {
		rr := rec.RR()
		if rr.Name == name && rr.Type == recordType && rr.Data == data {
			// Extract ID from the NameSilo record wrapper
			if nsRec, ok := rec.(namesileoRecord); ok {
				return nsRec.ID
			}
		}
	}
	return ""
}

// Helper method to find record ID by name and type (first match)
func (p *Provider) findRecordIDByNameType(records []libdns.Record, name, recordType string) string {
	for _, rec := range records {
		rr := rec.RR()
		if rr.Name == name && rr.Type == recordType {
			// Extract ID from the NameSilo record wrapper
			if nsRec, ok := rec.(namesileoRecord); ok {
				return nsRec.ID
			}
		}
	}
	return ""
}

// doHTTPRequest performs an HTTP request and unmarshals the XML response
func (p *Provider) doHTTPRequest(client *http.Client, req *http.Request, resp interface{}) error {
	response, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(response.Body)
		return fmt.Errorf("unexpected HTTP status %d: %s", response.StatusCode, string(respBody))
	}

	result, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if err := xml.Unmarshal(result, resp); err != nil {
		return fmt.Errorf("failed to unmarshal XML response: %w", err)
	}

	return nil
}

// Interface guards
var (
	_ libdns.RecordGetter   = (*Provider)(nil)
	_ libdns.RecordAppender = (*Provider)(nil)
	_ libdns.RecordSetter   = (*Provider)(nil)
	_ libdns.RecordDeleter  = (*Provider)(nil)
)
