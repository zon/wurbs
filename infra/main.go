package main

import (
	"crypto/rand"
	"encoding/hex"
	"os"

	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes"
	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/apiextensions"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "wurbs")

		kubeconfig := cfg.Get("kubeconfig")
		if kubeconfig == "" {
			kubeconfig = os.Getenv("KUBECONFIG")
		}

		k8s, err := kubernetes.NewProvider(ctx, "k8s", &kubernetes.ProviderArgs{
			EnableServerSideApply: pulumi.Bool(true),
			Kubeconfig:            pulumi.String(kubeconfig),
		})
		if err != nil {
			return err
		}

		_, err = corev1.NewNamespace(ctx, "ralph-wurbs", &corev1.NamespaceArgs{
			Metadata: &metav1.ObjectMetaArgs{
				Name: pulumi.String("ralph-wurbs"),
			},
		}, pulumi.Provider(k8s))
		if err != nil {
			return err
		}

		wurbsNs, err := corev1.NewNamespace(ctx, "wurbs", &corev1.NamespaceArgs{
			Metadata: &metav1.ObjectMetaArgs{
				Name: pulumi.String("wurbs"),
			},
		}, pulumi.Provider(k8s))
		if err != nil {
			return err
		}

		password := generatePassword(16)
		username := "postgres"

		_, err = corev1.NewSecret(ctx, "postgres-secret", &corev1.SecretArgs{
			Metadata: &metav1.ObjectMetaArgs{
				Name:      pulumi.String("postgres-creds"),
				Namespace: wurbsNs.Metadata.Name(),
			},
			StringData: pulumi.StringMap{
				"username": pulumi.String(username),
				"password": pulumi.String(password),
			},
		}, pulumi.Provider(k8s), pulumi.Parent(wurbsNs))
		if err != nil {
			return err
		}

		_, err = apiextensions.NewCustomResource(ctx, "postgres-cluster", &apiextensions.CustomResourceArgs{
			ApiVersion: pulumi.String("postgresql.cnpg.io/v1"),
			Kind:       pulumi.String("Cluster"),
			Metadata: &metav1.ObjectMetaArgs{
				Name:      pulumi.String("wurbs-postgres"),
				Namespace: wurbsNs.Metadata.Name(),
			},
			OtherFields: map[string]interface{}{
				"spec": map[string]interface{}{
					"instances": 3,
					"bootstrap": map[string]interface{}{
						"initdb": map[string]interface{}{
							"database": "wurbs",
							"owner":    "wurbs",
						},
					},
					"storage": map[string]interface{}{
						"size": "1Gi",
					},
				},
			},
		}, pulumi.Provider(k8s), pulumi.Parent(wurbsNs))
		if err != nil {
			return err
		}

		return nil
	})
}

func generatePassword(length int) string {
	bytes := make([]byte, length)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)[:length]
}
