package services

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"log"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
)

var (
	entropyMu sync.Mutex
	entropy   = ulid.Monotonic(rand.Reader, 0)
)

func NewULID() string {
	entropyMu.Lock()
	defer entropyMu.Unlock()
	t := time.Now().UTC()
	id, err := ulid.New(ulid.Timestamp(t), entropy)
	if err != nil {
		entropy = ulid.Monotonic(rand.Reader, 0)
		id = ulid.MustNew(ulid.Timestamp(t), entropy)
	}
	return id.String()
}

func GenerateAPIKey() string {
	b := make([]byte, 32)
	rand.Read(b)
	return "pos_" + hex.EncodeToString(b)
}

func normalizeSyncFields(data map[string]interface{}, entityType string) {
	if data == nil {
		return
	}

	if _, ok := data["local_id"]; !ok {
		if id, ok := data["id"]; ok {
			data["local_id"] = id
		}
	}

	switch entityType {
	case "order":
		if _, ok := data["status"]; !ok {
			if v, ok := data["order_status"]; ok {
				data["status"] = v
			}
		}
		if _, ok := data["customer_name"]; !ok {
			if v, ok := data["created_by"]; ok {
				data["customer_name"] = v
			}
		}
		if _, ok := data["created_at"]; !ok {
			data["created_at"] = time.Now().UTC().Format(time.RFC3339)
		}
		if _, ok := data["updated_at"]; !ok {
			data["updated_at"] = time.Now().UTC().Format(time.RFC3339)
		}

	case "transaction":
		if _, ok := data["created_at"]; !ok {
			if v, ok := data["transaction_date"]; ok {
				data["created_at"] = v
			}
		}
		if _, ok := data["cashier_name"]; !ok {
			if v, ok := data["created_by"]; ok {
				data["cashier_name"] = v
			}
		}
	}
}

func parseTime(s string) interface{} {
	if s == "" {
		return sql.NullTime{}
	}

	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05", // ISO tanpa zona (mis. transaction_date dari app)
		"2006-01-02T15:04:05.999999",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}

	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}

	// Fallback diam-diam ke "sekarang" menggeser data ke tanggal sync — minimal
	// tinggalkan jejak supaya timestamp korup dari client bisa terdeteksi.
	log.Printf("parseTime: format tidak dikenali %q, fallback ke NOW", s)
	return time.Now().UTC()
}

// parseTimeStrict parses s and reports whether it was a valid timestamp.
// Unlike parseTime, it does NOT fall back to now() — empty/invalid returns ok=false
// so callers can choose a better fallback (e.g. the linked order's created_at).
func parseTimeStrict(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05", // ISO tanpa zona (mis. transaction_date dari app)
		"2006-01-02T15:04:05.999999",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func nullStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
