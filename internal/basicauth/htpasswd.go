package basicauth

import (
	"bufio"
	"errors"
	"fmt"
	"hash/maphash"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/AB-Lindex/rest-rego/internal/types"
	"golang.org/x/crypto/bcrypt"
)

// ErrNoValidCredentials is returned when a file contains no usable bcrypt entries.
var ErrNoValidCredentials = errors.New("basicauth: no valid credentials found in file")

// minAllowedCost is the lowest bcrypt cost accepted when loading credentials (REQ-007).
const (
	minAllowedCost  = 10
	defaultCacheTTL = 2 * time.Minute
)

// loadFile parses an Apache 2.4 htpasswd file and returns a credential map.
// Only bcrypt hashes ($2y$, $2b$, $2a$) are accepted; all other formats are
// logged at WARN level and skipped. Returns ErrNoValidCredentials if the
// resulting map is empty.
func loadFile(filePath string) (*credMap, error) {
	f, err := os.Open(filePath) // #nosec G304 — path comes from operator-controlled config
	if err != nil {
		return nil, fmt.Errorf("basicauth: cannot open file %q: %w", filePath, err)
	}
	defer f.Close()

	creds := make(credMap)
	scanner := bufio.NewScanner(f)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		colonIdx := strings.IndexByte(line, ':')
		if colonIdx < 0 {
			slog.Warn("basicauth: skipping line with no colon", "file", filePath, "line", lineNum)
			continue
		}

		username := line[:colonIdx]
		hash := line[colonIdx+1:]

		switch {
		case strings.HasPrefix(hash, "$2y$"), strings.HasPrefix(hash, "$2b$"), strings.HasPrefix(hash, "$2a$"):
			cost, err := bcrypt.Cost([]byte(hash))
			if err != nil {
				slog.Warn("basicauth: skipping entry with invalid bcrypt hash", "file", filePath, "line", lineNum)
				continue
			}
			if cost < minAllowedCost {
				slog.Warn("basicauth: skipping entry — bcrypt cost too low (minimum 10)",
					"file", filePath, "line", lineNum, "cost", strconv.Itoa(cost))
				continue
			}
			if cost < 12 {
				slog.Warn("basicauth: bcrypt cost below recommended minimum of 12",
					"file", filePath, "line", lineNum, "cost", strconv.Itoa(cost))
			}
			creds[username] = hash

		case strings.HasPrefix(hash, "$apr1$"):
			slog.Warn("basicauth: skipping MD5 hash (unsupported)", "file", filePath, "line", lineNum)

		case strings.HasPrefix(hash, "{SHA}"):
			slog.Warn("basicauth: skipping SHA-1 hash (unsupported)", "file", filePath, "line", lineNum)

		default:
			slog.Warn("basicauth: skipping unsupported hash format", "file", filePath, "line", lineNum)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("basicauth: error reading file %q: %w", filePath, err)
	}

	if len(creds) == 0 {
		return nil, ErrNoValidCredentials
	}

	slog.Info("basicauth: loaded credentials", "count", len(creds), "file", filePath)
	return &creds, nil
}

func (b *BasicAuthProvider) createCacheKey(user, password string) uint64 {
	var h maphash.Hash
	h.SetSeed(b.seed) // make sure to re-use the same seed for consistent hashing across calls
	h.WriteString(user)
	h.WriteByte(':')
	h.WriteString(password)

	return h.Sum64()
}

// verifyPassword checks a bcrypt hash against the given plaintext password.
func (b *BasicAuthProvider) verifyPassword(user, hash, password string) error {
	cacheKey := b.createCacheKey(user, password)
	if b.cache != nil {
		if valid, found := b.cache.Get(cacheKey); found {
			// Cache hit for invalid credentials, treat as authentication failure.
			if !valid {
				return types.ErrAuthenticationFailed
			}
			// Cache hit for valid credentials, skip bcrypt check.
			return nil
		}
	}

	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if b.cache != nil {
		b.cache.SetWithTTL(cacheKey, err == nil, 1, defaultCacheTTL)
	}
	return err
}
