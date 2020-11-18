package monitor

import (
	"github.com/go-orm/gorm"
	"github.com/lileio/pubsub/v2/providers/memory"
	"github.com/niedbalski/go-athena/pkg/common"
	"github.com/niedbalski/go-athena/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"testing"
)

var defaultTestConfig = `
monitor:
  api-key: "adc017510b7dec542da7494558d6567f42be0a8fa958497aef0b968738ab2222"
  filetypes:
      - "sosreport-*"
  directories:
      - "/uploads"
      - "/uploads/sosreport"

  processor-map:
    - type: filename
      regex: ".*sosreport.*$"
      processor: sosreports

    - type: case
      regex: 00295561
      processor: process-00295561

processor:
  subscribe-to:
    - sosreports
    - kernel
`

type MonitorTestSuite struct {
	suite.Suite
	config *config.Config
	db *gorm.DB
}

type TestSalesforceClient struct {
	common.BaseSalesforceClient
}

func (sf *TestSalesforceClient) GetCaseByNumber(number string) (*common.Case, error) {
	return nil, nil
}


func (s *MonitorTestSuite) SetupTest() {
	s.config, _ = config.NewConfigFromBytes([]byte(defaultTestConfig))
	s.db, _ = gorm.Open("sqlite3", "file::memory:?cache=shared")
	s.db.AutoMigrate(common.File{})
}


func (s *MonitorTestSuite) TestRunMonitor() {
	filesClient, err := common.NewFilesComClient(func(apikey string, dirs []string) ([]common.File, error) {
		return []common.File{
			common.File{Path: "/uploads/sosreport-testing-1.tar.xz"},
			common.File{Path: "/uploads/sosreport-testing-2.tar.xz"},
			common.File{Path: "/uploads/sosreport-testing-3.tar.xz"},
		}, nil
	}, s.config.Monitor.APIKey)

	assert.Nil(s.T(), err)
	provider := &memory.MemoryProvider{}
	monitor, err := NewMonitor(filesClient, &TestSalesforceClient{}, provider, s.config, s.db)
	assert.Nil(s.T(), err)

	err = monitor.Run()

	assert.Nil(s.T(), err)
	assert.Equal(s.T(), len(provider.Msgs), 3)
}

func TestMonitor(t *testing.T) {
	suite.Run(t, &MonitorTestSuite{})
}