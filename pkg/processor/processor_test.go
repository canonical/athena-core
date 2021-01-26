package processor

import (
	"context"
	"encoding/json"
	"github.com/lileio/pubsub/v2"
	"github.com/lileio/pubsub/v2/providers/memory"
	"github.com/niedbalski/go-athena/pkg/common"
	"github.com/niedbalski/go-athena/pkg/common/test"
	"github.com/niedbalski/go-athena/pkg/config"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"io/ioutil"
	"testing"
	"time"
)

type ProcessorTestSuite struct {
	suite.Suite
	config *config.Config
}

func init() {
	logrus.SetOutput(ioutil.Discard)
}

func (s *ProcessorTestSuite) SetupTest() {
	s.config, _ = config.NewConfigFromBytes([]byte(test.DefaultTestConfig))
}

type MockSubscriber struct {
	mock.Mock
	Options pubsub.HandlerOptions
}

func (s *MockSubscriber) Setup(c *pubsub.Client) {
	c.On(s.Options)
}

func (s *ProcessorTestSuite) TestRunProcessor() {
	filesComClient := test.TestFilesComClient{}
	salesforceClient := test.TestSalesforceClient{}
	pastebinClient := test.TestPastebinClient{}

	provider := &memory.MemoryProvider{}
	processor, _ := NewProcessor(&filesComClient, &salesforceClient, &pastebinClient, provider, s.config)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	b, _ := json.Marshal(common.File{Path: "/uploads/sosreport-123.tar.xz"})
	b1, _ := json.Marshal(common.File{Path: "/uploads/kernel-crashdump.tar.xz"})

	_ = provider.Publish(context.Background(), "sosreports", &pubsub.Msg{Data: b})
	_ = provider.Publish(context.Background(), "kernel", &pubsub.Msg{Data: b1})

	var called int = 0

	processor.Run(ctx, func(fc common.FilesComClient, sf common.SalesforceClient, pb common.PastebinClient,
		name string, topic string, reports map[string]config.Report, cfg *config.Config) pubsub.Subscriber {
		var subscriber = MockSubscriber{Options: pubsub.HandlerOptions{
			Topic:   topic,
			Name:    "athena-processor-" + name,
			AutoAck: true,
			JSON:    true,
		}}

		subscriber.Options.Handler = func(ctx context.Context, msg *common.File, m *pubsub.Msg) error {
			called++
			return nil
		}
		return &subscriber
	})

	assert.Equal(s.T(), called, 2)
	assert.Equal(s.T(), len(provider.Msgs), 2)
}

func TestNewProcessor(t *testing.T) {
	suite.Run(t, &ProcessorTestSuite{})
}
