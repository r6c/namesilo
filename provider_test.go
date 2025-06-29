package namesilo

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/libdns/libdns"
)

var (
	APIToken = os.Getenv("LIBDNS_NAMESILO_TOKEN")
	zone     = os.Getenv("LIBDNS_NAMESILO_ZONE")
)

var (
	testRecords []libdns.Record
)

func TestAppendRecords(t *testing.T) {
	if APIToken == "" {
		t.Skip("LIBDNS_NAMESILO_TOKEN not set")
	}
	if zone == "" {
		t.Skip("LIBDNS_NAMESILO_ZONE not set")
	}

	provider := Provider{APIToken: APIToken}
	ctx := context.Background()

	newRecords := []libdns.Record{
		libdns.CNAME{
			Name:   "test898008",
			Target: "wikipedia.com.",
			TTL:    time.Hour,
		},
		libdns.TXT{
			Name: "test289808",
			Text: "test value for namesilo",
			TTL:  time.Hour,
		},
	}

	records, err := provider.AppendRecords(ctx, zone, newRecords)
	if err != nil {
		t.Fatalf("AppendRecords failed: %v", err)
	}

	if len(newRecords) != len(records) {
		t.Errorf("Expected %d records, got %d", len(newRecords), len(records))
	}

	// Store for cleanup
	testRecords = append(testRecords, records...)

	t.Logf("Successfully added %d records", len(records))
}

func TestGetRecords(t *testing.T) {
	if APIToken == "" {
		t.Skip("LIBDNS_NAMESILO_TOKEN not set")
	}
	if zone == "" {
		t.Skip("LIBDNS_NAMESILO_ZONE not set")
	}

	provider := Provider{APIToken: APIToken}
	ctx := context.Background()

	records, err := provider.GetRecords(ctx, zone)
	if err != nil {
		t.Fatalf("GetRecords failed: %v", err)
	}

	if len(records) == 0 {
		t.Error("No records found")
	}

	t.Logf("Found %d records in zone %s", len(records), zone)

	// Log first few records for debugging
	for i, record := range records {
		if i >= 3 { // Only show first 3 records
			break
		}
		rr := record.RR()
		t.Logf("Record %d: %s %s %s TTL=%v", i+1, rr.Name, rr.Type, rr.Data, rr.TTL)
	}
}

func TestSetRecords(t *testing.T) {
	if APIToken == "" {
		t.Skip("LIBDNS_NAMESILO_TOKEN not set")
	}
	if zone == "" {
		t.Skip("LIBDNS_NAMESILO_ZONE not set")
	}

	provider := Provider{APIToken: APIToken}
	ctx := context.Background()

	// Test updating existing records and adding new ones
	records := []libdns.Record{
		libdns.TXT{
			Name: "test652753",
			Text: "new test value for set operation",
			TTL:  2 * time.Hour,
		},
		libdns.CNAME{
			Name:   "test289808-new",
			Target: "example.com.",
			TTL:    time.Hour,
		},
	}

	resultRecords, err := provider.SetRecords(ctx, zone, records)
	if err != nil {
		t.Fatalf("SetRecords failed: %v", err)
	}

	if len(records) != len(resultRecords) {
		t.Errorf("Expected %d records, got %d", len(records), len(resultRecords))
	}

	// Store for cleanup
	testRecords = append(testRecords, resultRecords...)

	t.Logf("Successfully set %d records", len(resultRecords))
}

func TestDeleteRecords(t *testing.T) {
	if APIToken == "" {
		t.Skip("LIBDNS_NAMESILO_TOKEN not set")
	}
	if zone == "" {
		t.Skip("LIBDNS_NAMESILO_ZONE not set")
	}

	// Skip if no test records to delete
	if len(testRecords) == 0 {
		t.Skip("No test records to delete")
	}

	provider := Provider{APIToken: APIToken}
	ctx := context.Background()

	deletedRecords, err := provider.DeleteRecords(ctx, zone, testRecords)
	if err != nil {
		t.Fatalf("DeleteRecords failed: %v", err)
	}

	t.Logf("Successfully deleted %d records", len(deletedRecords))

	// Clear test records
	testRecords = nil
}

func TestRecordTypes(t *testing.T) {
	if APIToken == "" {
		t.Skip("LIBDNS_NAMESILO_TOKEN not set")
	}
	if zone == "" {
		t.Skip("LIBDNS_NAMESILO_ZONE not set")
	}

	provider := Provider{APIToken: APIToken}
	ctx := context.Background()

	// Test different record types
	testRecords := []libdns.Record{
		libdns.TXT{
			Name: "test-txt",
			Text: "v=spf1 include:_spf.example.com ~all",
			TTL:  time.Hour,
		},
		libdns.MX{
			Name:       "test-mx",
			Target:     "mail.example.com.",
			Preference: 10,
			TTL:        time.Hour,
		},
	}

	// Add records
	addedRecords, err := provider.AppendRecords(ctx, zone, testRecords)
	if err != nil {
		t.Fatalf("Failed to add test records: %v", err)
	}

	// Verify they were added
	allRecords, err := provider.GetRecords(ctx, zone)
	if err != nil {
		t.Fatalf("Failed to get records: %v", err)
	}

	found := 0
	for _, record := range allRecords {
		rr := record.RR()
		if rr.Name == "test-txt" && rr.Type == "TXT" {
			found++
		}
		if rr.Name == "test-mx" && rr.Type == "MX" {
			found++
		}
	}

	if found != len(testRecords) {
		t.Errorf("Expected to find %d test records, found %d", len(testRecords), found)
	}

	// Clean up
	_, err = provider.DeleteRecords(ctx, zone, addedRecords)
	if err != nil {
		t.Logf("Warning: Failed to clean up test records: %v", err)
	}

	t.Logf("Successfully tested %d record types", len(testRecords))
}

func TestErrorHandling(t *testing.T) {
	// Test with invalid API token
	provider := Provider{APIToken: "invalid-token"}
	ctx := context.Background()

	_, err := provider.GetRecords(ctx, "example.com")
	if err == nil {
		t.Error("Expected error with invalid API token")
	}

	// Test with empty API token
	provider = Provider{APIToken: ""}
	_, err = provider.GetRecords(ctx, "example.com")
	if err == nil {
		t.Error("Expected error with empty API token")
	}

	t.Log("Error handling tests passed")
}
