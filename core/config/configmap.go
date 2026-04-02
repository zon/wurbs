package config

import (
	"github.com/zon/chat/core/k8s"
	"gopkg.in/yaml.v3"
)

var applyConfigmapFunc = k8s.ApplyConfigmap

const (
	configMapName          = "wurbs"
	ralphWorkflowNamespace = "ralph-wurbs"
)

type ConfigMap struct {
	RESTPort         int    `yaml:"restPort"`
	SocketPort       int    `yaml:"socketPort"`
	OIDCIssuer       string `yaml:"oidcIssuer"`
	OIDCClientID     string `yaml:"oidcClientID"`
	OIDCClientSecret string `yaml:"oidcClientSecret"`
	NATSURL          string `yaml:"natsURL"`
}

func (c *ConfigMap) Load() error {
	return LoadYAML(c)
}

func (c *ConfigMap) Write() error {
	tree, err := Dir()
	if err != nil {
		return err
	}
	return saveYAML(tree.Config, c)
}

func (c *ConfigMap) WriteToK8s(context string) error {
	data, err := c.MarshalConfigMap()
	if err != nil {
		return err
	}
	return applyConfigmapFunc(configMapName, ralphWorkflowNamespace, context, data)
}

func (c *ConfigMap) MarshalConfigMap() (map[string]string, error) {
	data, err := yaml.Marshal(c)
	if err != nil {
		return nil, err
	}
	return map[string]string{
		"config.yaml": string(data),
	}, nil
}
