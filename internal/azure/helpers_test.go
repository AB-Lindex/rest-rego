package azure

import (
	"testing"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwt"
)

func TestGetTokenString(t *testing.T) {
	testCases := []struct {
		name          string
		setupToken    func() jwt.Token
		field         string
		expectedValue string
	}{
		{
			name: "get string field from token",
			setupToken: func() jwt.Token {
				token := jwt.New()
				_ = token.Set("sub", "user123")
				return token
			},
			field:         "sub",
			expectedValue: "user123",
		},
		{
			name: "get email field from token",
			setupToken: func() jwt.Token {
				token := jwt.New()
				_ = token.Set("email", "user@example.com")
				return token
			},
			field:         "email",
			expectedValue: "user@example.com",
		},
		{
			name: "get name field from token",
			setupToken: func() jwt.Token {
				token := jwt.New()
				_ = token.Set("name", "John Doe")
				return token
			},
			field:         "name",
			expectedValue: "John Doe",
		},
		{
			name: "field does not exist",
			setupToken: func() jwt.Token {
				token := jwt.New()
				_ = token.Set("sub", "user123")
				return token
			},
			field:         "nonexistent",
			expectedValue: "",
		},
		{
			name: "field exists but is not a string - integer",
			setupToken: func() jwt.Token {
				token := jwt.New()
				_ = token.Set("count", 42)
				return token
			},
			field:         "count",
			expectedValue: "",
		},
		{
			name: "field exists but is not a string - boolean",
			setupToken: func() jwt.Token {
				token := jwt.New()
				_ = token.Set("verified", true)
				return token
			},
			field:         "verified",
			expectedValue: "",
		},
		{
			name: "field exists but is not a string - array",
			setupToken: func() jwt.Token {
				token := jwt.New()
				_ = token.Set("roles", []string{"admin", "user"})
				return token
			},
			field:         "roles",
			expectedValue: "",
		},
		{
			name: "field exists but is not a string - map",
			setupToken: func() jwt.Token {
				token := jwt.New()
				_ = token.Set("metadata", map[string]interface{}{"key": "value"})
				return token
			},
			field:         "metadata",
			expectedValue: "",
		},
		{
			name: "empty string value",
			setupToken: func() jwt.Token {
				token := jwt.New()
				_ = token.Set("empty", "")
				return token
			},
			field:         "empty",
			expectedValue: "",
		},
		{
			name: "string with spaces",
			setupToken: func() jwt.Token {
				token := jwt.New()
				_ = token.Set("description", "This is a test value")
				return token
			},
			field:         "description",
			expectedValue: "This is a test value",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			token := tc.setupToken()
			result := getTokenString(token, tc.field)

			if result != tc.expectedValue {
				t.Errorf("Expected %q, got %q", tc.expectedValue, result)
			}
		})
	}
}

func TestGetTokenString_StandardClaims(t *testing.T) {
	token := jwt.New()
	_ = token.Set(jwt.SubjectKey, "user123")
	_ = token.Set(jwt.IssuerKey, "https://issuer.example.com")
	_ = token.Set(jwt.JwtIDKey, "jwt-id-12345")
	_ = token.Set(jwt.IssuedAtKey, time.Now().Unix())
	_ = token.Set(jwt.ExpirationKey, time.Now().Add(time.Hour).Unix())

	tests := []struct {
		field    string
		expected string
	}{
		{jwt.SubjectKey, "user123"},
		{jwt.IssuerKey, "https://issuer.example.com"},
		{jwt.JwtIDKey, "jwt-id-12345"},
	}

	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			result := getTokenString(token, tt.field)
			if result != tt.expected {
				t.Errorf("For field %s: expected %q, got %q", tt.field, tt.expected, result)
			}
		})
	}

	t.Run("numeric claims return empty", func(t *testing.T) {
		iat := getTokenString(token, jwt.IssuedAtKey)
		if iat != "" {
			t.Errorf("Expected empty string for numeric IssuedAt, got %q", iat)
		}

		exp := getTokenString(token, jwt.ExpirationKey)
		if exp != "" {
			t.Errorf("Expected empty string for numeric Expiration, got %q", exp)
		}
	})
}
