package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"gorm.io/gorm"
)

// clientClaims holds the claims extracted from a client credential token.
type clientClaims struct {
	Subject string `json:"sub"`
	Email   string `json:"email"`
}

// GenerateRSAKeyPair generates a 2048-bit RSA key pair and returns the
// PEM-encoded private key and public key as strings.
func GenerateRSAKeyPair() (privateKeyPEM, publicKeyPEM string, err error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", fmt.Errorf("auth: failed to generate RSA key: %w", err)
	}

	privBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}
	privateKeyPEM = string(pem.EncodeToMemory(privBlock))

	pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return "", "", fmt.Errorf("auth: failed to marshal public key: %w", err)
	}
	pubBlock := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubDER,
	}
	publicKeyPEM = string(pem.EncodeToMemory(pubBlock))

	return privateKeyPEM, publicKeyPEM, nil
}

// parseRSAPublicKey parses a PEM-encoded RSA public key.
func parseRSAPublicKey(pemStr string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("auth: no PEM block found in public key")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("auth: failed to parse public key: %w", err)
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("auth: key is not an RSA public key")
	}
	return rsaPub, nil
}

// validateClientToken validates a JWT signed with the client RSA key and
// returns the extracted claims.
func validateClientToken(tokenStr string, pubKey *rsa.PublicKey) (*clientClaims, error) {
	tok, err := jwt.ParseSigned(tokenStr, []jose.SignatureAlgorithm{jose.RS256})
	if err != nil {
		return nil, fmt.Errorf("auth: failed to parse client token: %w", err)
	}

	var allClaims jwt.Claims
	var custom clientClaims
	if err := tok.Claims(pubKey, &allClaims, &custom); err != nil {
		return nil, fmt.Errorf("auth: client token signature invalid: %w", err)
	}

	// Validate expiry.
	expected := jwt.Expected{
		Time: time.Now(),
	}
	if err := allClaims.Validate(expected); err != nil {
		return nil, fmt.Errorf("auth: client token validation failed: %w", err)
	}

	custom.Subject = allClaims.Subject
	if custom.Subject == "" && custom.Email == "" {
		return nil, fmt.Errorf("auth: client token missing subject and email claims")
	}

	return &custom, nil
}

// resolveClientUser finds a user by email from client credential claims.
func resolveClientUser(db *gorm.DB, claims *clientClaims) (*User, error) {
	var user User

	if claims.Email != "" {
		result := db.Where("email = ?", claims.Email).First(&user)
		if result.Error == nil {
			return &user, nil
		}
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, result.Error
		}
	}

	if claims.Subject != "" {
		result := db.Where("subject = ?", claims.Subject).First(&user)
		if result.Error == nil {
			return &user, nil
		}
		if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, result.Error
		}
	}

	return nil, fmt.Errorf("auth: user not found for client credentials")
}

func parseRSAPrivateKey(pemStr string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("auth: no PEM block found in private key")
	}
	priv, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("auth: failed to parse private key: %w", err)
	}
	return priv, nil
}

type clientClaimsJSON struct {
	Email string `json:"email,omitempty"`
}

func SignClientToken(privateKeyPEM, email, subject string) (string, error) {
	privKey, err := parseRSAPrivateKey(privateKeyPEM)
	if err != nil {
		return "", err
	}

	stdClaims := jwt.Claims{
		Subject:  subject,
		IssuedAt: jwt.NewNumericDate(time.Now()),
		Expiry:   jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
	}

	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: privKey},
		(&jose.SignerOptions{}).WithType("JWT"),
	)
	if err != nil {
		return "", fmt.Errorf("auth: failed to create signer: %w", err)
	}

	builder := jwt.Signed(signer).Claims(stdClaims)
	if email != "" {
		builder = builder.Claims(clientClaimsJSON{Email: email})
	}
	token, err := builder.Serialize()
	if err != nil {
		return "", fmt.Errorf("auth: failed to sign token: %w", err)
	}

	return token, nil
}
