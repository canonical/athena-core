package config

import (
	"github.com/makyo/snuffler"
	"gopkg.in/yaml.v3"
)

type Report struct {
	Command   string `yaml:"command"`
	Timeout   string `yaml:"timeout" default:"0s"`
	Script    string `yaml:"script"`
	ExitCodes string `yaml:"exit-codes" default:"any"`
}

type Subscriber struct {
	Topic             string            `yaml:"topic"`
	SFCommentEnabled  bool              `yaml:"sf-comment-enabled"`
	SFCommentIsPublic bool              `yaml:"sf-comment-public" default:"false"`
	SFComment         string            `yaml:"sf-comment"`
	Reports           map[string]Report `yaml:"reports"`
}

type Config struct {
	Monitor struct {
		APIKey       string   `yaml:"api-key"`
		DBPath       string   `yaml:"db-path" default:"."`
		PollEvery    string   `yaml:"poll-every" default:"5"`
		Filetypes    []string `yaml:"filetypes"`
		Directories  []string `yaml:"directories"`
		ProcessorMap []struct {
			Type      string `yaml:"type"`
			Regex     string `yaml:"regex"`
			Processor string `yaml:"processor"`
		} `yaml:"processor-map"`
	} `yaml:"monitor,omitempty"`
	Processor struct {
		BaseTmpDir  string                `yaml:"base-tmpdir" default:""`
		SubscribeTo map[string]Subscriber `yaml:"subscribers,omitempty"`
	} `yaml:"processor,omitempty"`
	Salesforce struct {
		Endpoint      string `yaml:"endpoint"`
		Username      string `yaml:"username"`
		Password      string `yaml:"password"`
		SecurityToken string `yaml:"security-token"`
	} `yaml:"salesforce,omitempty"`
	Pastebin struct {
		Key      string `yaml:"key"`
		Provider string `yaml:"provider"`
	} `yaml:"pastebin,omitempty"`
}

func NewConfigFromFile(filePaths []string) (*Config, error) {
	var config Config

	s := snuffler.New(&config)
	for _, filepath := range filePaths {
		if err := s.AddFile(filepath); err != nil {
			return nil, err
		}
	}

	if err := s.Snuffle(); err != nil {
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
