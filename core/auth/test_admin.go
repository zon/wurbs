package auth

import (
	"os"

	"github.com/zon/chat/core/k8s"
	"gopkg.in/yaml.v3"
)

// TestAdmin holds the credentials for a test administrator.
type TestAdmin struct {
	Email      string `yaml:"email"`
	PublicKey  string `yaml:"publicKey"`
	PrivateKey string `yaml:"privateKey"`
}

// ReadK8s populates the TestAdmin from a Kubernetes secret.
func (t *TestAdmin) ReadK8s(name, namespace, context string) error {
	data, err := k8s.GetSecret(name, namespace, context)
	if err != nil {
		return err
	}
	t.Email = data["email"]
	t.PublicKey = data["publicKey"]
	t.PrivateKey = data["privateKey"]
	return nil
}

// WriteK8s applies the TestAdmin as a Kubernetes secret.
func (t *TestAdmin) WriteK8s(name, namespace, context string) error {
	return k8s.ApplySecret(name, namespace, context, map[string]string{
		"email":      t.Email,
		"publicKey":  t.PublicKey,
		"privateKey": t.PrivateKey,
	})
}

// Write serializes the TestAdmin to a YAML file at path.
func (t *TestAdmin) Write(path string) error {
	data, err := yaml.Marshal(t)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// Read deserializes the TestAdmin from a YAML file at path.
func (t *TestAdmin) Read(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, t)
}

var testMode bool

func SetTestMode(enabled bool) {
	testMode = enabled
}

// testClientPublicKey is a shared RSA public key used for client credential
// authentication in test mode. All test users can authenticate using tokens
// signed with the corresponding private key.
const testClientPublicKey = `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAu1SU1LfVLPHCozMxH2Mo
4lgOEePzNm0tRgeLezV6ffAt0gunVTLw7onLRnrq0/IzW7yWR7QkrmBL7jTKEn5u
+qKhbwKfBstIs+bMY2Zkp18gnTxKLxoS2tFczGkPLPgizskuemMghRniWaoLcyeh
kd3qqGElvW/VDL5AaWTg0nLVkjRo9z+40RQzuVaE8AkAFmxZzow3x+VJYKdjykkJ
0iT9wCS0DRTXu269V264Vf/3jvredZiKRkgwlL9xNAwxXFg0x/XFw005UWVRIkdg
cKWTjpBP2dPwVZ4WWC+9aGVd+Gyn1o0CLelf4rEjGoXbAAEgAqeGUxrcIlbjXfbcw
IDAQAB
-----END PUBLIC KEY-----`
