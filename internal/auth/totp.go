package auth

import (
	"strings"

	"github.com/gigliofr/gokit/authx/totp"
)

// GenerateTOTPSecret crea il segreto 2FA e l'URI otpauth:// (gokit/authx/totp).
func GenerateTOTPSecret(accountName, issuer string) (secret string, provisioningURI string, err error) {
	if strings.TrimSpace(issuer) == "" {
		issuer = "Visto Easy"
	}
	return totp.Generate(accountName, issuer)
}

// ValidateTOTP verifica un codice a 6 cifre (30s, tolleranza ±1 periodo).
func ValidateTOTP(code, secret string) bool { return totp.Validate(code, secret) }
