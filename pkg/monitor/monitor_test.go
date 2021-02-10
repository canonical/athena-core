package monitor

import (
	"context"
	"github.com/go-orm/gorm"
	_ "github.com/go-orm/gorm/dialects/sqlite"
	"github.com/lileio/pubsub/v2/providers/memory"
	"github.com/niedbalski/go-athena/pkg/common/db"
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
	s.db.AutoMigrate(db.File{}, db.Report{})
}

func (s *MonitorTestSuite) TestRunMonitor() {
	provider := &memory.MemoryProvider{}
	monitor, err := NewMonitor(&test.FilesComClient{}, &test.SalesforceClient{}, provider, s.config, s.db)
	assert.Nil(s.T(), err)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = monitor.Run(ctx)
	assert.NotZero(s.T(), len(provider.Msgs["sosreports"]))
}

func TestMonitor(t *testing.T) {
	suite.Run(t, &MonitorTestSuite{})
}
