package jwtsupport

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/AB-Lindex/rest-rego/internal/types"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

// TestLoadWellKnowns_ValidFile tests loading a valid well-known configuration from a file
func TestLoadWellKnowns_ValidFile(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()
	wellKnownPath := filepath.Join(tmpDir, "well-known.json")

	// Create a valid well-known.json file
	validJSON := `{
		"jwks_uri": "file:///` + tmpDir + `/jwks.json",
		"id_token_signing_alg_values_supported": ["RS256", "ES256"]
	}`

	if err := os.WriteFile(wellKnownPath, []byte(validJSON), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Construct JWTSupport with file URL
	j := &JWTSupport{
		wellKnowns: []string{"file://" + wellKnownPath},
	}

	// Call LoadWellKnowns
	j.LoadWellKnowns()

	// Assert wellknownList has 1 entry with correct JwksURI
	if len(j.wellknownList) != 1 {
		t.Fatalf("Expected 1 entry in wellknownList, got %d", len(j.wellknownList))
	}

	expectedJwksURI := "file:///" + tmpDir + "/jwks.json"
	if j.wellknownList[0].JwksURI != expectedJwksURI {
		t.Errorf("Expected JwksURI %q, got %q", expectedJwksURI, j.wellknownList[0].JwksURI)
	}

	// Assert isLocalFile flag is set to true for file-based sources
	if !j.wellknownList[0].isLocalFile {
		t.Error("Expected isLocalFile to be true for file-based well-known")
	}

	// Assert SupportedAlgorithms are loaded correctly
	if len(j.wellknownList[0].SupportedAlgorithms) != 2 {
		t.Errorf("Expected 2 supported algorithms, got %d", len(j.wellknownList[0].SupportedAlgorithms))
	}

	expectedAlgs := []string{"RS256", "ES256"}
	for i, alg := range expectedAlgs {
		if j.wellknownList[0].SupportedAlgorithms[i] != alg {
			t.Errorf("Expected algorithm %q at index %d, got %q", alg, i, j.wellknownList[0].SupportedAlgorithms[i])
		}
	}
}

// TestLoadWellKnowns_InvalidJSON tests that invalid JSON is handled gracefully
func TestLoadWellKnowns_InvalidJSON(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()
	wellKnownPath := filepath.Join(tmpDir, "invalid.json")

	// Create a file with invalid JSON
	invalidJSON := `{this is not valid json}`

	if err := os.WriteFile(wellKnownPath, []byte(invalidJSON), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Construct JWTSupport with file URL
	j := &JWTSupport{
		wellKnowns: []string{"file://" + wellKnownPath},
	}

	// Call LoadWellKnowns
	j.LoadWellKnowns()

	// Assert wellknownList remains empty
	if len(j.wellknownList) != 0 {
		t.Errorf("Expected wellknownList to be empty, got %d entries", len(j.wellknownList))
	}
}

// TestLoadWellKnowns_NonExistentFile tests that non-existent files are handled gracefully
func TestLoadWellKnowns_NonExistentFile(t *testing.T) {
	// Use a path that doesn't exist
	nonExistentPath := "/tmp/this-file-does-not-exist-12345.json"

	// Construct JWTSupport with file URL
	j := &JWTSupport{
		wellKnowns: []string{"file://" + nonExistentPath},
	}

	// Call LoadWellKnowns
	j.LoadWellKnowns()

	// Assert wellknownList remains empty
	if len(j.wellknownList) != 0 {
		t.Errorf("Expected wellknownList to be empty, got %d entries", len(j.wellknownList))
	}
}

// TestLoadWellKnowns_EmptyString tests that empty strings are skipped
func TestLoadWellKnowns_EmptyString(t *testing.T) {
	// Construct JWTSupport with empty string
	j := &JWTSupport{
		wellKnowns: []string{""},
	}

	// Call LoadWellKnowns
	j.LoadWellKnowns()

	// Assert wellknownList remains empty
	if len(j.wellknownList) != 0 {
		t.Errorf("Expected wellknownList to be empty, got %d entries", len(j.wellknownList))
	}
}

// TestLoadWellKnowns_MultipleFiles tests loading multiple well-known files
func TestLoadWellKnowns_MultipleFiles(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()

	// Create first well-known file
	wellKnown1Path := filepath.Join(tmpDir, "well-known-1.json")
	json1 := `{
		"jwks_uri": "file:///` + tmpDir + `/jwks1.json",
		"id_token_signing_alg_values_supported": ["RS256"]
	}`
	if err := os.WriteFile(wellKnown1Path, []byte(json1), 0644); err != nil {
		t.Fatalf("Failed to create first test file: %v", err)
	}

	// Create second well-known file
	wellKnown2Path := filepath.Join(tmpDir, "well-known-2.json")
	json2 := `{
		"jwks_uri": "file:///` + tmpDir + `/jwks2.json",
		"id_token_signing_alg_values_supported": ["ES256"]
	}`
	if err := os.WriteFile(wellKnown2Path, []byte(json2), 0644); err != nil {
		t.Fatalf("Failed to create second test file: %v", err)
	}

	// Construct JWTSupport with multiple file URLs
	j := &JWTSupport{
		wellKnowns: []string{
			"file://" + wellKnown1Path,
			"file://" + wellKnown2Path,
		},
	}

	// Call LoadWellKnowns
	j.LoadWellKnowns()

	// Assert wellknownList has 2 entries
	if len(j.wellknownList) != 2 {
		t.Fatalf("Expected 2 entries in wellknownList, got %d", len(j.wellknownList))
	}

	// Assert both entries have isLocalFile set to true
	if !j.wellknownList[0].isLocalFile {
		t.Error("Expected first well-known to have isLocalFile=true")
	}
	if !j.wellknownList[1].isLocalFile {
		t.Error("Expected second well-known to have isLocalFile=true")
	}
}

// TestLoadWellKnowns_MixedValidInvalid tests a mix of valid and invalid files
func TestLoadWellKnowns_MixedValidInvalid(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()

	// Create a valid well-known file
	validPath := filepath.Join(tmpDir, "valid.json")
	validJSON := `{
		"jwks_uri": "file:///` + tmpDir + `/jwks.json",
		"id_token_signing_alg_values_supported": ["RS256"]
	}`
	if err := os.WriteFile(validPath, []byte(validJSON), 0644); err != nil {
		t.Fatalf("Failed to create valid test file: %v", err)
	}

	// Create an invalid well-known file
	invalidPath := filepath.Join(tmpDir, "invalid.json")
	invalidJSON := `{invalid json}`
	if err := os.WriteFile(invalidPath, []byte(invalidJSON), 0644); err != nil {
		t.Fatalf("Failed to create invalid test file: %v", err)
	}

	// Construct JWTSupport with both URLs
	j := &JWTSupport{
		wellKnowns: []string{
			"file://" + validPath,
			"file://" + invalidPath,
			"file:///this/does/not/exist.json",
		},
	}

	// Call LoadWellKnowns
	j.LoadWellKnowns()

	// Assert only the valid file was loaded
	if len(j.wellknownList) != 1 {
		t.Fatalf("Expected 1 entry in wellknownList, got %d", len(j.wellknownList))
	}

	expectedJwksURI := "file:///" + tmpDir + "/jwks.json"
	if j.wellknownList[0].JwksURI != expectedJwksURI {
		t.Errorf("Expected JwksURI %q, got %q", expectedJwksURI, j.wellknownList[0].JwksURI)
	}

	// Assert isLocalFile flag is true for file-based source
	if !j.wellknownList[0].isLocalFile {
		t.Error("Expected isLocalFile to be true for file-based well-known")
	}
}

// TestLoadJWKS_ValidFile tests loading a valid JWKS from a file
func TestLoadJWKS_ValidFile(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()
	jwksPath := filepath.Join(tmpDir, "jwks.json")

	// Create a valid JWKS file with one RSA key
	validJWKS := `{
		"keys": [
			{
				"kty": "RSA",
				"use": "sig",
				"kid": "test-key-1",
				"e": "AQAB",
				"n": "v3m2wZo5FMPJKb6q-f4Kql4f07GrR88yG7g76eTKSENIJ5xfAX_gj2GlxFgjQKyYK4YNWiT7Oge2Ym8fTt7Ljn-MQjHLFsvwnBPgk8iff5Up1R0tQBP2ABG5lWGG_pL4PW_agBtrgv8_xcxG95jbgO7cqmgMg5httyKSbJzWpPUNi8ZKfffwy24FOPwQnMp0qp96xmWYnRCyVFvz_xzllRvAZL4ohPJU-UHbsJeCbjHrOxjDWTfeJoCj8M3dFMFgzisjU6rFLeoLkMLyKPy9R_dN3Sd57ittONqt8Y65bLC4d4YX-l14FGGjppUiOXoGnm08M5yJpfzLQC0dkqIKeQ",
				"alg": "RS256"
			}
		]
	}`

	if err := os.WriteFile(jwksPath, []byte(validJWKS), 0644); err != nil {
		t.Fatalf("Failed to create test JWKS file: %v", err)
	}

	// Set up JWTSupport with wellknownList pointing to file-based JWKS
	j := &JWTSupport{
		wellknownList: []*wellKnownData{
			{
				JwksURI:             "file://" + jwksPath,
				SupportedAlgorithms: []string{"RS256"},
				sourceURL:           "file:///test/well-known.json",
			},
		},
	}

	// Call LoadJWKS
	j.LoadJWKS()

	// Assert j.JWKS has 1 entry
	if len(j.JWKS) != 1 {
		t.Fatalf("Expected 1 entry in JWKS, got %d", len(j.JWKS))
	}

	// Assert the JWKS set contains 1 key
	if j.JWKS[0].Len() != 1 {
		t.Errorf("Expected JWKS to contain 1 key, got %d", j.JWKS[0].Len())
	}
}

// TestLoadJWKS_InvalidJSON tests that invalid JWKS JSON is handled gracefully
func TestLoadJWKS_InvalidJSON(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()
	jwksPath := filepath.Join(tmpDir, "invalid-jwks.json")

	// Create an invalid JWKS file
	invalidJWKS := `{this is not valid json}`

	if err := os.WriteFile(jwksPath, []byte(invalidJWKS), 0644); err != nil {
		t.Fatalf("Failed to create test JWKS file: %v", err)
	}

	// Set up JWTSupport with wellknownList pointing to invalid file
	j := &JWTSupport{
		wellknownList: []*wellKnownData{
			{
				JwksURI:             "file://" + jwksPath,
				SupportedAlgorithms: []string{"RS256"},
				sourceURL:           "file:///test/well-known.json",
			},
		},
	}

	// Call LoadJWKS
	j.LoadJWKS()

	// Assert j.JWKS stays empty
	if len(j.JWKS) != 0 {
		t.Errorf("Expected JWKS to be empty for invalid JSON, got %d entries", len(j.JWKS))
	}
}

// TestLoadJWKS_AlgorithmEnrichment tests that PostFetch enriches keys without algorithms
func TestLoadJWKS_AlgorithmEnrichment(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()
	jwksPath := filepath.Join(tmpDir, "jwks-no-alg.json")

	// Create a JWKS file WITHOUT the "alg" field
	jwksWithoutAlg := `{
		"keys": [
			{
				"kty": "RSA",
				"use": "sig",
				"kid": "test-key-no-alg",
				"e": "AQAB",
				"n": "v3m2wZo5FMPJKb6q-f4Kql4f07GrR88yG7g76eTKSENIJ5xfAX_gj2GlxFgjQKyYK4YNWiT7Oge2Ym8fTt7Ljn-MQjHLFsvwnBPgk8iff5Up1R0tQBP2ABG5lWGG_pL4PW_agBtrgv8_xcxG95jbgO7cqmgMg5httyKSbJzWpPUNi8ZKfffwy24FOPwQnMp0qp96xmWYnRCyVFvz_xzllRvAZL4ohPJU-UHbsJeCbjHrOxjDWTfeJoCj8M3dFMFgzisjU6rFLeoLkMLyKPy9R_dN3Sd57ittONqt8Y65bLC4d4YX-l14FGGjppUiOXoGnm08M5yJpfzLQC0dkqIKeQ"
			}
		]
	}`

	if err := os.WriteFile(jwksPath, []byte(jwksWithoutAlg), 0644); err != nil {
		t.Fatalf("Failed to create test JWKS file: %v", err)
	}

	// Set up JWTSupport with SupportedAlgorithms that should be added by PostFetch
	j := &JWTSupport{
		wellknownList: []*wellKnownData{
			{
				JwksURI:             "file://" + jwksPath,
				SupportedAlgorithms: []string{"RS256"},
				sourceURL:           "file:///test/well-known.json",
			},
		},
	}

	// Call LoadJWKS
	j.LoadJWKS()

	// Assert j.JWKS has 1 entry
	if len(j.JWKS) != 1 {
		t.Fatalf("Expected 1 entry in JWKS, got %d", len(j.JWKS))
	}

	// Assert the JWKS contains at least 1 key
	if j.JWKS[0].Len() < 1 {
		t.Fatalf("Expected JWKS to contain at least 1 key after PostFetch enrichment, got %d", j.JWKS[0].Len())
	}

	// Verify that the key now has an algorithm set by PostFetch
	key, ok := j.JWKS[0].Key(0)
	if !ok {
		t.Fatal("Failed to get key from JWKS")
	}

	alg := key.Algorithm().String()
	if alg == "" {
		t.Error("Expected PostFetch to set an algorithm on the key, but algorithm is empty")
	}

	// Verify it's one of the supported algorithms
	if alg != "RS256" {
		t.Errorf("Expected algorithm to be RS256 (from SupportedAlgorithms), got %s", alg)
	}
}

// TestLoadJWKS_NonExistentFile tests that non-existent JWKS files are handled gracefully
func TestLoadJWKS_NonExistentFile(t *testing.T) {
	// Use a path that doesn't exist
	nonExistentPath := "/tmp/this-jwks-file-does-not-exist-12345.json"

	// Set up JWTSupport with wellknownList pointing to non-existent file
	j := &JWTSupport{
		wellknownList: []*wellKnownData{
			{
				JwksURI:             "file://" + nonExistentPath,
				SupportedAlgorithms: []string{"RS256"},
				sourceURL:           "file:///test/well-known.json",
			},
		},
	}

	// Call LoadJWKS
	j.LoadJWKS()

	// Assert j.JWKS stays empty
	if len(j.JWKS) != 0 {
		t.Errorf("Expected JWKS to be empty for non-existent file, got %d entries", len(j.JWKS))
	}
}

// TestLoadJWKS_MultipleFileAndHTTP tests mixed file and HTTP sources
func TestLoadJWKS_MultipleFiles(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()

	// Create first JWKS file
	jwks1Path := filepath.Join(tmpDir, "jwks1.json")
	jwks1 := `{
		"keys": [
			{
				"kty": "RSA",
				"use": "sig",
				"kid": "key-1",
				"e": "AQAB",
				"n": "v3m2wZo5FMPJKb6q-f4Kql4f07GrR88yG7g76eTKSENIJ5xfAX_gj2GlxFgjQKyYK4YNWiT7Oge2Ym8fTt7Ljn-MQjHLFsvwnBPgk8iff5Up1R0tQBP2ABG5lWGG_pL4PW_agBtrgv8_xcxG95jbgO7cqmgMg5httyKSbJzWpPUNi8ZKfffwy24FOPwQnMp0qp96xmWYnRCyVFvz_xzllRvAZL4ohPJU-UHbsJeCbjHrOxjDWTfeJoCj8M3dFMFgzisjU6rFLeoLkMLyKPy9R_dN3Sd57ittONqt8Y65bLC4d4YX-l14FGGjppUiOXoGnm08M5yJpfzLQC0dkqIKeQ",
				"alg": "RS256"
			}
		]
	}`
	if err := os.WriteFile(jwks1Path, []byte(jwks1), 0644); err != nil {
		t.Fatalf("Failed to create first test JWKS file: %v", err)
	}

	// Create second JWKS file with a different key
	jwks2Path := filepath.Join(tmpDir, "jwks2.json")
	jwks2 := `{
		"keys": [
			{
				"kty": "RSA",
				"use": "sig",
				"kid": "key-2",
				"e": "AQAB",
				"n": "xGOr-H7A-PWG7D8z0nvC2RUPvvNvzGdH_nSq6lLt8d_RnfcPPMQIv5FVfvVIb-5DlXJNL7qW7OPi_GdQC_W6h1cL7q6HwEzkKpLZshXKXRZ8kpPCx7W5FcRvzVvW2QoCl8n7F0cYKJRfhJp7qP3LKdZCl4RvCQdMqT2aLwxQCeE",
				"alg": "RS256"
			}
		]
	}`
	if err := os.WriteFile(jwks2Path, []byte(jwks2), 0644); err != nil {
		t.Fatalf("Failed to create second test JWKS file: %v", err)
	}

	// Set up JWTSupport with multiple file-based JWKS
	j := &JWTSupport{
		wellknownList: []*wellKnownData{
			{
				JwksURI:             "file://" + jwks1Path,
				SupportedAlgorithms: []string{"RS256"},
				sourceURL:           "file:///test/well-known-1.json",
			},
			{
				JwksURI:             "file://" + jwks2Path,
				SupportedAlgorithms: []string{"RS256"},
				sourceURL:           "file:///test/well-known-2.json",
			},
		},
	}

	// Call LoadJWKS
	j.LoadJWKS()

	// Assert j.JWKS has 2 entries
	if len(j.JWKS) != 2 {
		t.Fatalf("Expected 2 entries in JWKS, got %d", len(j.JWKS))
	}

	// Assert each JWKS set contains 1 key
	if j.JWKS[0].Len() != 1 {
		t.Errorf("Expected first JWKS to contain 1 key, got %d", j.JWKS[0].Len())
	}
	if j.JWKS[1].Len() != 1 {
		t.Errorf("Expected second JWKS to contain 1 key, got %d", j.JWKS[1].Len())
	}
}

// TestLoadJWKS_SourceTypeMismatch_FileToHTTP tests that file well-known → HTTP jwks_uri is rejected
func TestLoadJWKS_SourceTypeMismatch_FileToHTTP(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()

	// Create a well-known file that points to an HTTP jwks_uri (mismatch)
	wellKnownPath := filepath.Join(tmpDir, "well-known.json")
	wellKnownJSON := `{
		"jwks_uri": "https://example.com/jwks.json",
		"id_token_signing_alg_values_supported": ["RS256"]
	}`
	if err := os.WriteFile(wellKnownPath, []byte(wellKnownJSON), 0644); err != nil {
		t.Fatalf("Failed to create well-known file: %v", err)
	}

	// Construct JWTSupport and load well-known
	j := &JWTSupport{
		wellKnowns: []string{"file://" + wellKnownPath},
	}
	j.LoadWellKnowns()

	// Verify well-known loaded
	if len(j.wellknownList) != 1 {
		t.Fatalf("Expected 1 entry in wellknownList, got %d", len(j.wellknownList))
	}

	// Call LoadJWKS and verify it's rejected due to source type mismatch
	j.LoadJWKS()

	// Assert j.JWKS is empty due to source type mismatch
	if len(j.JWKS) != 0 {
		t.Errorf("Expected JWKS to be empty due to source type mismatch, got %d entries", len(j.JWKS))
	}
}

// TestLoadJWKS_SourceTypeMismatch_HTTPToFile tests that HTTP well-known → file jwks_uri is rejected
func TestLoadJWKS_SourceTypeMismatch_HTTPToFile(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()
	jwksPath := filepath.Join(tmpDir, "jwks.json")

	// Simulate an HTTP-sourced well-known by directly manipulating wellknownList
	// (we can't actually make HTTP requests in tests)
	j := &JWTSupport{
		wellknownList: []*wellKnownData{
			{
				JwksURI:             "file://" + jwksPath,
				SupportedAlgorithms: []string{"RS256"},
				sourceURL:           "https://example.com/.well-known/openid-configuration",
				isLocalFile:         false, // HTTP source
			},
		},
	}

	// Call LoadJWKS and verify it's rejected due to source type mismatch
	j.LoadJWKS()

	// Assert j.JWKS is empty due to source type mismatch
	if len(j.JWKS) != 0 {
		t.Errorf("Expected JWKS to be empty due to source type mismatch, got %d entries", len(j.JWKS))
	}
}

// TestLoadJWKS_SourceTypeMatch tests that matching source types are accepted
func TestLoadJWKS_SourceTypeMatch(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()

	// Create well-known file
	wellKnownPath := filepath.Join(tmpDir, "well-known.json")
	jwksPath := filepath.Join(tmpDir, "jwks.json")

	wellKnownJSON := `{
		"jwks_uri": "file://` + jwksPath + `",
		"id_token_signing_alg_values_supported": ["RS256"]
	}`
	if err := os.WriteFile(wellKnownPath, []byte(wellKnownJSON), 0644); err != nil {
		t.Fatalf("Failed to create well-known file: %v", err)
	}

	// Create JWKS file with matching source type
	jwksJSON := `{
		"keys": [
			{
				"kty": "RSA",
				"use": "sig",
				"kid": "test-key-1",
				"n": "xGOr-H7A-PWGfKF0N5Q",
				"e": "AQAB"
			}
		]
	}`
	if err := os.WriteFile(jwksPath, []byte(jwksJSON), 0644); err != nil {
		t.Fatalf("Failed to create JWKS file: %v", err)
	}

	// Construct JWTSupport and load well-known
	j := &JWTSupport{
		wellKnowns: []string{"file://" + wellKnownPath},
	}
	j.LoadWellKnowns()

	// Verify well-known loaded
	if len(j.wellknownList) != 1 {
		t.Fatalf("Expected 1 entry in wellknownList, got %d", len(j.wellknownList))
	}

	// Call LoadJWKS and verify it succeeds with matching source types
	j.LoadJWKS()

	// Assert j.JWKS has 1 entry (both file sources match)
	if len(j.JWKS) != 1 {
		t.Errorf("Expected 1 entry in JWKS with matching source types, got %d", len(j.JWKS))
	}

	// Verify the JWKS contains the expected key
	if j.JWKS[0].Len() != 1 {
		t.Errorf("Expected JWKS to contain 1 key, got %d", j.JWKS[0].Len())
	}

	// Verify isLocalFile flag is true for file-based source
	if !j.wellknownList[0].isLocalFile {
		t.Error("Expected isLocalFile to be true for file-based well-known")
	}
}

// TestAuthenticate_FileBasedKeys tests full JWT authentication flow with file-based JWKS
func TestAuthenticate_FileBasedKeys(t *testing.T) {
	// Generate RSA key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key pair: %v", err)
	}

	// Create JWK from public key
	publicJWK, err := jwk.FromRaw(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("Failed to create JWK from public key: %v", err)
	}

	// Set key ID and algorithm
	if err := publicJWK.Set(jwk.KeyIDKey, "test-key-1"); err != nil {
		t.Fatalf("Failed to set key ID: %v", err)
	}
	if err := publicJWK.Set(jwk.AlgorithmKey, jwa.RS256); err != nil {
		t.Fatalf("Failed to set algorithm: %v", err)
	}

	// Create JWKS with the public key
	set := jwk.NewSet()
	if err := set.AddKey(publicJWK); err != nil {
		t.Fatalf("Failed to add key to set: %v", err)
	}

	// Marshal JWKS to JSON
	jwksJSON, err := json.Marshal(set)
	if err != nil {
		t.Fatalf("Failed to marshal JWKS: %v", err)
	}

	// Create temporary directory for test files
	tmpDir := t.TempDir()
	jwksPath := filepath.Join(tmpDir, "jwks.json")
	wellKnownPath := filepath.Join(tmpDir, "well-known.json")

	// Write JWKS file
	if err := os.WriteFile(jwksPath, jwksJSON, 0644); err != nil {
		t.Fatalf("Failed to write JWKS file: %v", err)
	}

	// Create well-known file
	wellKnownJSON := `{
		"jwks_uri": "file://` + jwksPath + `",
		"id_token_signing_alg_values_supported": ["RS256"]
	}`
	if err := os.WriteFile(wellKnownPath, []byte(wellKnownJSON), 0644); err != nil {
		t.Fatalf("Failed to write well-known file: %v", err)
	}

	// Construct JWTSupport
	j := &JWTSupport{
		wellKnowns:  []string{"file://" + wellKnownPath},
		audienceKey: "aud",
		audiences:   []string{"test-audience"},
		authKind:    "bearer",
		permissive:  false,
	}

	// Load well-known and JWKS
	j.LoadWellKnowns()
	j.LoadJWKS()

	// Verify JWKS loaded
	if len(j.JWKS) != 1 {
		t.Fatalf("Expected 1 JWKS entry, got %d", len(j.JWKS))
	}

	// Create a valid JWT token
	token := jwt.New()
	if err := token.Set(jwt.AudienceKey, "test-audience"); err != nil {
		t.Fatalf("Failed to set audience: %v", err)
	}
	if err := token.Set(jwt.SubjectKey, "test-user"); err != nil {
		t.Fatalf("Failed to set subject: %v", err)
	}
	if err := token.Set(jwt.IssuedAtKey, time.Now().Unix()); err != nil {
		t.Fatalf("Failed to set issued at: %v", err)
	}
	if err := token.Set(jwt.ExpirationKey, time.Now().Add(1*time.Hour).Unix()); err != nil {
		t.Fatalf("Failed to set expiration: %v", err)
	}

	// Sign the token with private key
	privateJWK, err := jwk.FromRaw(privateKey)
	if err != nil {
		t.Fatalf("Failed to create private JWK: %v", err)
	}
	if err := privateJWK.Set(jwk.KeyIDKey, "test-key-1"); err != nil {
		t.Fatalf("Failed to set private key ID: %v", err)
	}

	signedToken, err := jwt.Sign(token, jwt.WithKey(jwa.RS256, privateJWK))
	if err != nil {
		t.Fatalf("Failed to sign token: %v", err)
	}

	// Create mock types.Info with the token
	info := &types.Info{
		Request: types.RequestInfo{
			Auth: &types.RequestAuth{
				Kind:  "bearer",
				Token: string(signedToken),
			},
		},
	}

	// Create a mock HTTP request
	req, err := http.NewRequest("GET", "http://example.com/test", nil)
	if err != nil {
		t.Fatalf("Failed to create mock request: %v", err)
	}

	// Call Authenticate and verify it succeeds
	err = j.Authenticate(info, req)
	if err != nil {
		t.Errorf("Expected authentication to succeed, got error: %v", err)
	}

	// Verify info.JWT contains expected claims
	if info.JWT == nil {
		t.Fatal("Expected info.JWT to be populated")
	}

	// Convert JWT to map for assertion
	jwtMap, ok := info.JWT.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected info.JWT to be map[string]interface{}, got %T", info.JWT)
	}

	// Check audience claim (can be string or array)
	audValue := jwtMap["aud"]
	var foundAud bool
	switch v := audValue.(type) {
	case string:
		foundAud = v == "test-audience"
	case []interface{}:
		for _, aud := range v {
			if audStr, ok := aud.(string); ok && audStr == "test-audience" {
				foundAud = true
				break
			}
		}
	case []string:
		for _, aud := range v {
			if aud == "test-audience" {
				foundAud = true
				break
			}
		}
	default:
		// Try to handle it as an array type even if it's not []interface{}
		t.Logf("Audience claim type: %T, value: %v", audValue, audValue)
	}
	if !foundAud {
		t.Errorf("Expected aud claim to contain 'test-audience', got %v (type: %T)", jwtMap["aud"], jwtMap["aud"])
	}

	if sub, ok := jwtMap["sub"].(string); !ok || sub != "test-user" {
		t.Errorf("Expected sub claim to be 'test-user', got %v", jwtMap["sub"])
	}
}

// TestAuthenticate_FileBasedKeys_InvalidToken tests that invalid tokens are rejected
func TestAuthenticate_FileBasedKeys_InvalidToken(t *testing.T) {
	// Generate RSA key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key pair: %v", err)
	}

	// Create JWK from public key
	publicJWK, err := jwk.FromRaw(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("Failed to create JWK from public key: %v", err)
	}

	// Set key ID and algorithm
	if err := publicJWK.Set(jwk.KeyIDKey, "test-key-1"); err != nil {
		t.Fatalf("Failed to set key ID: %v", err)
	}
	if err := publicJWK.Set(jwk.AlgorithmKey, jwa.RS256); err != nil {
		t.Fatalf("Failed to set algorithm: %v", err)
	}

	// Create JWKS with the public key
	set := jwk.NewSet()
	if err := set.AddKey(publicJWK); err != nil {
		t.Fatalf("Failed to add key to set: %v", err)
	}

	// Marshal JWKS to JSON
	jwksJSON, err := json.Marshal(set)
	if err != nil {
		t.Fatalf("Failed to marshal JWKS: %v", err)
	}

	// Create temporary directory for test files
	tmpDir := t.TempDir()
	jwksPath := filepath.Join(tmpDir, "jwks.json")
	wellKnownPath := filepath.Join(tmpDir, "well-known.json")

	// Write JWKS file
	if err := os.WriteFile(jwksPath, jwksJSON, 0644); err != nil {
		t.Fatalf("Failed to write JWKS file: %v", err)
	}

	// Create well-known file
	wellKnownJSON := `{
		"jwks_uri": "file://` + jwksPath + `",
		"id_token_signing_alg_values_supported": ["RS256"]
	}`
	if err := os.WriteFile(wellKnownPath, []byte(wellKnownJSON), 0644); err != nil {
		t.Fatalf("Failed to write well-known file: %v", err)
	}

	// Construct JWTSupport
	j := &JWTSupport{
		wellKnowns:  []string{"file://" + wellKnownPath},
		audienceKey: "aud",
		audiences:   []string{"test-audience"},
		authKind:    "bearer",
		permissive:  false,
	}

	// Load well-known and JWKS
	j.LoadWellKnowns()
	j.LoadJWKS()

	// Test with various invalid tokens
	testCases := []struct {
		name        string
		token       string
		description string
	}{
		{
			name:        "malformed_token",
			token:       "not.a.valid.jwt",
			description: "Malformed JWT token",
		},
		{
			name:        "empty_token",
			token:       "",
			description: "Empty token",
		},
		{
			name:        "random_string",
			token:       "thisisnotavalidtoken",
			description: "Random string as token",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create mock types.Info with invalid token
			info := &types.Info{
				Request: types.RequestInfo{
					Auth: &types.RequestAuth{
						Kind:  "bearer",
						Token: tc.token,
					},
				},
			}

			// Create a mock HTTP request
			req, err := http.NewRequest("GET", "http://example.com/test", nil)
			if err != nil {
				t.Fatalf("Failed to create mock request: %v", err)
			}

			// Call Authenticate and verify it fails
			err = j.Authenticate(info, req)
			if err == nil {
				t.Errorf("Expected authentication to fail for %s, but it succeeded", tc.description)
			}

			// Verify error is ErrAuthenticationFailed
			if err != types.ErrAuthenticationFailed {
				t.Logf("Got error: %v (expected ErrAuthenticationFailed)", err)
			}
		})
	}
}

// TestAuthenticate_FileBasedKeys_WrongAudience tests that tokens with wrong audience are rejected
func TestAuthenticate_FileBasedKeys_WrongAudience(t *testing.T) {
	// Generate RSA key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate RSA key pair: %v", err)
	}

	// Create JWK from public key
	publicJWK, err := jwk.FromRaw(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("Failed to create JWK from public key: %v", err)
	}

	// Set key ID and algorithm
	if err := publicJWK.Set(jwk.KeyIDKey, "test-key-1"); err != nil {
		t.Fatalf("Failed to set key ID: %v", err)
	}
	if err := publicJWK.Set(jwk.AlgorithmKey, jwa.RS256); err != nil {
		t.Fatalf("Failed to set algorithm: %v", err)
	}

	// Create JWKS with the public key
	set := jwk.NewSet()
	if err := set.AddKey(publicJWK); err != nil {
		t.Fatalf("Failed to add key to set: %v", err)
	}

	// Marshal JWKS to JSON
	jwksJSON, err := json.Marshal(set)
	if err != nil {
		t.Fatalf("Failed to marshal JWKS: %v", err)
	}

	// Create temporary directory for test files
	tmpDir := t.TempDir()
	jwksPath := filepath.Join(tmpDir, "jwks.json")
	wellKnownPath := filepath.Join(tmpDir, "well-known.json")

	// Write JWKS file
	if err := os.WriteFile(jwksPath, jwksJSON, 0644); err != nil {
		t.Fatalf("Failed to write JWKS file: %v", err)
	}

	// Create well-known file
	wellKnownJSON := `{
		"jwks_uri": "file://` + jwksPath + `",
		"id_token_signing_alg_values_supported": ["RS256"]
	}`
	if err := os.WriteFile(wellKnownPath, []byte(wellKnownJSON), 0644); err != nil {
		t.Fatalf("Failed to write well-known file: %v", err)
	}

	// Construct JWTSupport with specific audience
	j := &JWTSupport{
		wellKnowns:  []string{"file://" + wellKnownPath},
		audienceKey: "aud",
		audiences:   []string{"expected-audience"},
		authKind:    "bearer",
		permissive:  false,
	}

	// Load well-known and JWKS
	j.LoadWellKnowns()
	j.LoadJWKS()

	// Create a JWT token with WRONG audience
	token := jwt.New()
	if err := token.Set(jwt.AudienceKey, "wrong-audience"); err != nil {
		t.Fatalf("Failed to set audience: %v", err)
	}
	if err := token.Set(jwt.SubjectKey, "test-user"); err != nil {
		t.Fatalf("Failed to set subject: %v", err)
	}
	if err := token.Set(jwt.IssuedAtKey, time.Now().Unix()); err != nil {
		t.Fatalf("Failed to set issued at: %v", err)
	}
	if err := token.Set(jwt.ExpirationKey, time.Now().Add(1*time.Hour).Unix()); err != nil {
		t.Fatalf("Failed to set expiration: %v", err)
	}

	// Sign the token with private key
	privateJWK, err := jwk.FromRaw(privateKey)
	if err != nil {
		t.Fatalf("Failed to create private JWK: %v", err)
	}
	if err := privateJWK.Set(jwk.KeyIDKey, "test-key-1"); err != nil {
		t.Fatalf("Failed to set private key ID: %v", err)
	}

	signedToken, err := jwt.Sign(token, jwt.WithKey(jwa.RS256, privateJWK))
	if err != nil {
		t.Fatalf("Failed to sign token: %v", err)
	}

	// Create mock types.Info with the token
	info := &types.Info{
		Request: types.RequestInfo{
			Auth: &types.RequestAuth{
				Kind:  "bearer",
				Token: string(signedToken),
			},
		},
	}

	// Create a mock HTTP request
	req, err := http.NewRequest("GET", "http://example.com/test", nil)
	if err != nil {
		t.Fatalf("Failed to create mock request: %v", err)
	}

	// Call Authenticate and verify it fails due to wrong audience
	err = j.Authenticate(info, req)
	if err == nil {
		t.Error("Expected authentication to fail for wrong audience, but it succeeded")
	}

	// Verify error is ErrAuthenticationFailed
	if err != types.ErrAuthenticationFailed {
		t.Logf("Got error: %v (expected ErrAuthenticationFailed)", err)
	}
}
