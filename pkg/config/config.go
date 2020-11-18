package config

import (
	"gopkg.in/yaml.v3"
	"io/ioutil"
)

type Config struct {
	Monitor struct {
		APIKey       string   `yaml:"api-key"`
		Filetypes    []string `yaml:"filetypes"`
		Directories  []string `yaml:"directories"`
		ProcessorMap []struct {
			Type      string      `yaml:"type"`
			Regex     string	  `yaml:"regex"`
			Processor string      `yaml:"processor"`
		} `yaml:"processor-map"`
	} `yaml:"monitor"`
	Processor struct {
		SubscribeTo []string `yaml:"subscribe-to"`
	} `yaml:"processor"`
	Salesforce struct {
		Endpoint      string `yaml:"endpoint"`
		Username      string `yaml:"username"`
		Password      string `yaml:"password"`
		SecurityToken string `yaml:"security-token"`
	} `yaml:"common"`
}

func NewConfigFromFile(filepath string) (*Config, error) {
	var config Config
	data, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func NewConfigFromBytes(data []byte) (*Config, error) {
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	return &config, nil
}
