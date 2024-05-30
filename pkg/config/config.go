package config

import (
	"github.com/makyo/snuffler"
	"gopkg.in/yaml.v3"
)

type Script struct {
	Timeout   string `yaml:"timeout" default:"0s"`
	ExitCodes string `yaml:"exit-codes" default:"any"`
	Run       string `yaml:"run"`
	RunScript string
}

type Report struct {
	Timeout string            `yaml:"timeout" default:"0s"`
	Scripts map[string]Script `yaml:"scripts"`
}

type Subscriber struct {
	Topic             string            `yaml:"topic"`
	SFCommentEnabled  bool              `yaml:"sf-comment-enabled"`
	SFCommentIsPublic bool              `yaml:"sf-comment-public" default:"false"`
	SFComment         string            `yaml:"sf-comment"`
	Reports           map[string]Report `yaml:"reports"`
}

type Db struct {
	Dialect string `yaml:"dialect" default:"sqlite"`
	DSN     string `yaml:"dsn"`
}

func NewDb() Db {
	return Db{
		Dialect: "sqlite",
	}
}

type Monitor struct {
	PollEvery    string   `yaml:"poll-every"`
	FilesDelta   string   `yaml:"files-delta"`
	Filetypes    []string `yaml:"filetypes"`
	BaseTmpDir   string   `yaml:"base-tmpdir"`
	Directories  []string `yaml:"directories"`
	ProcessorMap []struct {
		Type      string `yaml:"type"`
		Regex     string `yaml:"regex"`
		Processor string `yaml:"processor"`
	} `yaml:"processor-map"`
}

func NewMonitor() Monitor {
	return Monitor{
		PollEvery:  "5",
		FilesDelta: "10m",
	}
}

type Processor struct {
	ReportsUploadPath    string                `yaml:"reports-upload-dir"`
	BatchCommentsEvery   string                `yaml:"batch-comments-every"`
	BaseTmpDir           string                `yaml:"base-tmpdir"`
	KeepProcessingOutput bool                  `yaml:"keep-processing-output"`
	SubscribeTo          map[string]Subscriber `yaml:"subscribers,omitempty"`
}

func NewProcessor() Processor {
	return Processor{
		ReportsUploadPath:  "/customers/athena-reports/",
		BatchCommentsEvery: "10m",
	}
}

type SalesForce struct {
	Endpoint         string `yaml:"endpoint"`
	Username         string `yaml:"username"`
	Password         string `yaml:"password"`
	SecurityToken    string `yaml:"security-token"`
	MaxCommentLength int    `yaml:"max-comment-length"`
	EnableChatter    bool   `yaml:"enable-chatter"`
}

func NewSalesForce() SalesForce {
	return SalesForce{
		MaxCommentLength: 4000 - 1000, // A very conservative buffer of max length per Salesforce comment (4000) without header text for comments
		EnableChatter:    false,
	}
}

type Config struct {
	Db         Db         `yaml:"db,omitempty"`
	Monitor    Monitor    `yaml:"monitor,omitempty"`
	Processor  Processor  `yaml:"processor,omitempty"`
	Salesforce SalesForce `yaml:"salesforce,omitempty"`
	FilesCom   struct {
		Key      string `yaml:"key"`
		Endpoint string `yaml:"endpoint"`
	} `yaml:"filescom,omitempty"`
}

func NewConfig() Config {
	return Config{
		Db:         NewDb(),
		Monitor:    NewMonitor(),
		Processor:  NewProcessor(),
		Salesforce: NewSalesForce(),
	}
}

func (cfg *Config) String() string {
	tempCfg := *cfg
	// Sanitize output, i.e. remove sensitive information.
	tempCfg.Salesforce.Password = "**********"
	tempCfg.Salesforce.SecurityToken = "**********"
	tempCfg.FilesCom.Key = "**********"
	result, err := yaml.Marshal(tempCfg)
	if err != nil {
		return "could not marshal config"
	}
	return string(result)
}

func NewConfigFromFile(filePaths []string) (*Config, error) {
	var config Config = NewConfig()

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
