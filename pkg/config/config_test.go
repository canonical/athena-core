package config

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestNewDb(t *testing.T) {
	db := NewDb()

	if db.Dialect != "sqlite" {
		t.Errorf("Expected Dialect to be 'sqlite', got '%s'", db.Dialect)
	}

	if db.DSN != "" {
		t.Errorf("Expected DSN to be '', got '%s'", db.DSN)
	}
}

func TestNewMonitor(t *testing.T) {
	monitor := NewMonitor()

	if monitor.PollEvery != "5" {
		t.Errorf("Expected PollEvery to be '5', got '%s'", monitor.PollEvery)
	}

	if monitor.FilesDelta != "10m" {
		t.Errorf("Expected FilesDelta to be '10m', got '%s'", monitor.FilesDelta)
	}

	if len(monitor.Filetypes) != 0 {
		t.Errorf("Expected Filetypes to be empty, got '%v'", monitor.Filetypes)
	}

	if monitor.BaseTmpDir != "" {
		t.Errorf("Expected BaseTmpDir to be '', got '%s'", monitor.BaseTmpDir)
	}

	if len(monitor.Directories) != 0 {
		t.Errorf("Expected Directories to be empty, got '%v'", monitor.Directories)
	}

	if len(monitor.ProcessorMap) != 0 {
		t.Errorf("Expected ProcessorMap to be empty, got '%v'", monitor.ProcessorMap)
	}
}

func TestNewProcessor(t *testing.T) {
	processor := NewProcessor()

	if processor.ReportsUploadPath != "/customers/athena-reports/" {
		t.Errorf("Expected ReportsUploadPath to be '/customers/athena-reports/', got '%s'", processor.ReportsUploadPath)
	}

	if processor.BatchCommentsEvery != "10m" {
		t.Errorf("Expected BatchCommentsEvery to be '10m', got '%s'", processor.BatchCommentsEvery)
	}

	if processor.BaseTmpDir != "" {
		t.Errorf("Expected BaseTmpDir to be '', got '%s'", processor.BaseTmpDir)
	}

	if len(processor.SubscribeTo) != 0 {
		t.Errorf("Expected SubscribeTo to be empty, got '%v'", processor.SubscribeTo)
	}
}

func TestNewSalesforce(t *testing.T) {
	salesforce := NewSalesForce()

	if salesforce.Endpoint != "" {
		t.Errorf("Expected Endpoint to be '', got '%s'", salesforce.Endpoint)
	}

	if salesforce.Username != "" {
		t.Errorf("Expected Username to be '', got '%s'", salesforce.Username)
	}

	if salesforce.Password != "" {
		t.Errorf("Expected Password to be '', got '%s'", salesforce.Password)
	}

	if salesforce.SecurityToken != "" {
		t.Errorf("Expected SecurityToken to be '', got '%s'", salesforce.SecurityToken)
	}

	if salesforce.MaxCommentLength != 3000 {
		t.Errorf("Expected MaxCommentLength to be 3000, got '%d'", salesforce.MaxCommentLength)
	}
}

func TestNewConfigFromFile(t *testing.T) {
	// Create a temporary file
	tempFile, err := ioutil.TempFile("", "config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tempFile.Name()) // Clean up

	// Write some YAML content to the file
	yamlContent := `
db:
  dialect: postgres
monitor:
  poll-every: 10
  files-delta: 20m
`
	if _, err := tempFile.Write([]byte(yamlContent)); err != nil {
		t.Fatal(err)
	}
	if err := tempFile.Close(); err != nil {
		t.Fatal(err)
	}

	// Call NewConfigFromFile with the path of the temporary file
	config, err := NewConfigFromFile([]string{tempFile.Name()})
	if err != nil {
		t.Fatal(err)
	}

	// Check if the config has the expected values
	if config.Db.Dialect != "postgres" {
		t.Errorf("Expected Db.Dialect to be 'postgres', got '%s'", config.Db.Dialect)
	}

	if config.Monitor.PollEvery != "10" {
		t.Errorf("Expected Monitor.PollEvery to be '10', got '%s'", config.Monitor.PollEvery)
	}

	if config.Monitor.FilesDelta != "20m" {
		t.Errorf("Expected Monitor.FilesDelta to be '20m', got '%s'", config.Monitor.FilesDelta)
	}

	if config.Salesforce.MaxCommentLength != 3000 {
		t.Errorf("Expected MaxCommentLength to be 3000, got '%d'", config.Salesforce.MaxCommentLength)
	}
}
