package set

import (
	"github.com/zon/chat/core/k8s"
)

func GetClusterIP(context string) (string, error) {
	return k8s.GetClusterIP(context)
}

func GetSecret(name, namespace, context string) (map[string]string, error) {
	return k8s.GetSecret(name, namespace, context)
}

func ApplyConfigmap(name, namespace, context string, data map[string]string) error {
	return k8s.ApplyConfigmap(name, namespace, context, data)
}

func ApplySecret(name, namespace, context string, data map[string]string) error {
	return k8s.ApplySecret(name, namespace, context, data)
}

func WriteConfigmapFile(filename, name, namespace string, data map[string]string) error {
	return k8s.WriteConfigmapFile(filename, name, namespace, data)
}

func WriteSecretFile(filename, name, namespace string, data map[string]string) error {
	return k8s.WriteSecretFile(filename, name, namespace, data)
}
