# NameSilo DNS Provider for libdns

[![Go Reference](https://pkg.go.dev/badge/github.com/libdns/namesilo.svg)](https://pkg.go.dev/github.com/libdns/namesilo)

This package implements the [libdns interfaces](https://github.com/libdns/libdns) for [NameSilo](https://www.namesilo.com/), allowing you to manage DNS records programmatically.

## Features

- ✅ Get records (`GetRecords`)
- ✅ Add records (`AppendRecords`)
- ✅ Update records (`SetRecords`)
- ✅ Delete records (`DeleteRecords`)
- ✅ Supports all major DNS record types (A, AAAA, CNAME, MX, TXT, NS, SRV)
- ✅ Proper URL encoding and error handling
- ✅ TTL validation with NameSilo minimums

## Installation

```bash
go get github.com/libdns/namesilo
```

## Authentication

You'll need a NameSilo API token. You can get one from your [NameSilo API Manager](https://www.namesilo.com/account/api-manager).

## Usage

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/libdns/libdns"
	"github.com/libdns/namesilo"
)

func main() {
	provider := &namesilo.Provider{
		APIToken: "your-namesilo-api-token",
	}

	ctx := context.Background()
	zone := "example.com."

	// Get all records
	records, err := provider.GetRecords(ctx, zone)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Found %d records\n", len(records))

	// Add a new record
	newRecords := []libdns.Record{
		libdns.TXT{
			Name: "test",
			Text: "Hello from libdns!",
			TTL:  time.Hour,
		},
		libdns.CNAME{
			Name:   "www",
			Target: "example.com.",
			TTL:    2 * time.Hour,
		},
	}

	added, err := provider.AppendRecords(ctx, zone, newRecords)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Added %d records\n", len(added))

	// Update records (replace existing or create new)
	updated := []libdns.Record{
		libdns.TXT{
			Name: "test",
			Text: "Updated text record",
			TTL:  30 * time.Minute,
		},
	}

	set, err := provider.SetRecords(ctx, zone, updated)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Set %d records\n", len(set))

	// Delete records
	deleted, err := provider.DeleteRecords(ctx, zone, added)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Deleted %d records\n", len(deleted))
}
```

## Supported Record Types

| Type  | Supported | Notes |
|-------|-----------|-------|
| A     | ✅        | IPv4 addresses |
| AAAA  | ✅        | IPv6 addresses |
| CNAME | ✅        | Canonical names |
| MX    | ✅        | Mail exchange records with priority |
| TXT   | ✅        | Text records |
| NS    | ✅        | Name server records |
| SRV   | ✅        | Service records with priority, weight, port |

## Special Notes

### Record Names
- Use `@` for the zone root (e.g., `example.com`)
- Use relative names for subdomains (e.g., `www` for `www.example.com`)
- Absolute names ending with `.` are automatically converted to relative names

### TTL Handling
- NameSilo has a minimum TTL of 300 seconds (5 minutes)
- If you specify a TTL less than 300 seconds, it will be automatically set to 3600 seconds (1 hour)
- All TTL values are in seconds

### MX and SRV Records
- MX records use the `Preference` field for priority
- SRV records use `Priority`, `Weight`, and `Port` fields as expected

## Testing

To run the tests, set the following environment variables:

```bash
export LIBDNS_NAMESILO_TOKEN="your-api-token"
export LIBDNS_NAMESILO_ZONE="your-test-domain.com"
go test -v
```

**Warning**: The tests will create and delete real DNS records. Use a test domain that you don't mind modifying.

## API Rate Limits

NameSilo has API rate limits. This library includes:
- 30-second HTTP timeouts
- Proper error handling for rate limit responses
- Sequential record operations to avoid overwhelming the API

## Error Handling

The provider includes comprehensive error handling:
- Invalid API tokens return descriptive errors
- HTTP errors are properly wrapped and returned
- NameSilo API error codes are translated to meaningful messages
- Network timeouts are handled gracefully

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
