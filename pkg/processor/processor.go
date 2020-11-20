package processor

import (
	"context"
	"fmt"
	"github.com/lileio/pubsub/v2"
	"github.com/lileio/pubsub/v2/middleware/defaults"
	"github.com/niedbalski/go-athena/pkg/common"
	"github.com/niedbalski/go-athena/pkg/config"
	"os"
	"time"
)

type Processor struct {
	Config   *config.Config
	FilesClient *common.FilesComClient
	SalesforceClient common.SalesforceClient
	Provider pubsub.Provider
}


type BaseSubscriber struct {
	Options pubsub.HandlerOptions
}

func (s *BaseSubscriber) Setup(c *pubsub.Client) {
	c.On(s.Options)
}

func (s *BaseSubscriber) Handler(ctx context.Context, msg *common.File, m *pubsub.Msg) error {
	fmt.Printf("Message received %+v\n\n", m)
	time.Sleep(15*time.Second)
	return nil
}

func NewBaseSubscriber(name, topic string) (*BaseSubscriber) {
	var subscriber = BaseSubscriber{Options: pubsub.HandlerOptions{
		Topic:   topic,
		Name:    "athena-processor-" + name,
		AutoAck: true,
		JSON:    true,
	}}
	subscriber.Options.Handler = subscriber.Handler
	return &subscriber
}

func NewProcessor(filesClient *common.FilesComClient, salesforceClient common.SalesforceClient, provider pubsub.Provider, cfg *config.Config) (*Processor, error) {
	return &Processor{Provider: provider, FilesClient: filesClient, SalesforceClient: salesforceClient, Config: cfg}, nil
}

func (p *Processor) Run(ctx context.Context, newSubscriberFn func(name, topic string) pubsub.Subscriber) (error) {
	hostname, err := os.Hostname()
	if err != nil {
		return err
	}

	pubsub.SetClient(&pubsub.Client{
		ServiceName: "athena-processor",
		Provider:    p.Provider,
		Middleware:  defaults.Middleware,
	})

	for _, event := range p.Config.Processor.SubscribeTo {
		go pubsub.Subscribe(newSubscriberFn(hostname, event.Topic))
	}

	select {
	case <- ctx.Done():
		return nil
	}
}


