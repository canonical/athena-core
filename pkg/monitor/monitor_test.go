package monitor

import (
	"context"
	"github.com/go-orm/gorm"
	"github.com/lileio/pubsub/v2/providers/memory"
	"github.com/niedbalski/go-athena/pkg/common"
	"github.com/niedbalski/go-athena/pkg/common/test"
	"github.com/niedbalski/go-athena/pkg/config"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"io/ioutil"
	"testing"
	"time"
)

func init() {
	logrus.SetOutput(ioutil.Discard)
}

type MonitorTestSuite struct {
	suite.Suite
	config *config.Config
	db     *gorm.DB
}

func (s *MonitorTestSuite) SetupTest() {
	s.config, _ = config.NewConfigFromBytes([]byte(test.DefaultTestConfig))
	s.db, _ = gorm.Open("sqlite3", "file::memory:?cache=shared")
	s.db.AutoMigrate(common.File{})
}

func (s *MonitorTestSuite) TestRunMonitor() {
	provider := &memory.MemoryProvider{}
	monitor, err := NewMonitor(&test.TestFilesComClient{}, &test.TestSalesforceClient{}, provider, s.config, s.db)
	assert.Nil(s.T(), err)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err = monitor.Run(ctx)

	assert.Nil(s.T(), err)
	assert.NotZero(s.T(), len(provider.Msgs["sosreports"]))
}

func TestMonitor(t *testing.T) {
	suite.Run(t, &MonitorTestSuite{})
}
