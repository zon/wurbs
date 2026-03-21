package main

import (
	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes"
	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/apiextensions"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		k8s, err := kubernetes.NewProvider(ctx, "k8s", &kubernetes.ProviderArgs{
			EnableServerSideApply: pulumi.Bool(true),
			Context:               pulumi.String("microk8s"),
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

		_, err = corev1.NewService(ctx, "postgres-nodeport", &corev1.ServiceArgs{
			Metadata: &metav1.ObjectMetaArgs{
				Name:      pulumi.String("wurbs-postgres-nodeport"),
				Namespace: wurbsNs.Metadata.Name(),
			},
			Spec: &corev1.ServiceSpecArgs{
				Type: pulumi.String("NodePort"),
				Selector: pulumi.StringMap{
					"cnpg.io/cluster": pulumi.String("wurbs-postgres"),
					"role":            pulumi.String("primary"),
				},
				Ports: corev1.ServicePortArray{
					&corev1.ServicePortArgs{
						Port:       pulumi.Int(5432),
						TargetPort: pulumi.Any(5432),
						NodePort:   pulumi.Int(32432),
					},
				},
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
					"instances": 1,
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
