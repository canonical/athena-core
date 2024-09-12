package monitor

import (
	"context"
	"github.com/canonical/athena-core/pkg/common/db"
	"github.com/canonical/athena-core/pkg/common/test"
	"github.com/canonical/athena-core/pkg/config"
	"github.com/lileio/pubsub/v2/providers/memory"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"io"
	"testing"
	"time"
)

func init() {
	logrus.SetOutput(io.Discard)
}

type MonitorTestSuite struct {
	suite.Suite
	config *config.Config
	db     *gorm.DB
}

func (s *MonitorTestSuite) SetupTest() {
	s.config, _ = config.NewConfigFromBytes([]byte(test.DefaultTestConfig))
	assert.Equal(s.T(), "sqlite", s.config.Db.Dialect)
	s.db, _ = gorm.Open(sqlite.Open("file::memory:?cache=shared"))
	s.db.AutoMigrate(db.File{}, db.Report{})
}

func (s *MonitorTestSuite) TestRunMonitor() {
	provider := &memory.MemoryProvider{}
	monitor, err := NewMonitor(provider, s.config, s.db, &test.SalesforceClientFactory{}, &test.FilesComClientFactory{})
	assert.Nil(s.T(), err)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = monitor.Run(ctx)
	assert.NotZero(s.T(), len(provider.Msgs["sosreports"]))
}

func TestMonitor(t *testing.T) {
	suite.Run(t, &MonitorTestSuite{})
}
