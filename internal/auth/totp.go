package auth

import (
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

func GenerateTOTPSecret(accountName, issuer string) (secret string, provisioningURI string, err error) {
	if strings.TrimSpace(issuer) == "" {
		issuer = "Visto Easy"
	}
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: strings.TrimSpace(accountName),
	})
	if err != nil {
		return "", "", err
	}
	return key.Secret(), key.URL(), nil
}

func ValidateTOTP(code, secret string) bool {
	ok, err := totp.ValidateCustom(strings.TrimSpace(code), strings.TrimSpace(secret), time.Now().UTC(), totp.ValidateOpts{Period: 30, Skew: 1, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1})
	if err != nil {
		return false
	}
	return ok
}
