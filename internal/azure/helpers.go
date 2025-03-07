package azure

import (
	"github.com/lestrrat-go/jwx/v2/jwt"
)

func getTokenString(token jwt.Token, field string) string {
	v, ok := token.Get(field)
	if !ok {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
