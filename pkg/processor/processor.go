package processor

import (
	"context"
	"fmt"
	"github.com/lileio/pubsub/v2"
	"github.com/lileio/pubsub/v2/middleware/defaults"
	"github.com/lileio/pubsub/v2/providers/nats"
	"github.com/niedbalski/go-athena/cmd"
	"github.com/niedbalski/go-athena/pkg/config"
	"github.com/niedbalski/go-athena/pkg/common"
	"os"
	"os/signal"
	"syscall"
)

type Processor struct {
	Config           *config.Config
	SalesforceClient common.SalesforceClient
}

type Subscriber struct {
	Options pubsub.HandlerOptions
}

func (s *Subscriber) Setup(c *pubsub.Client) {
	c.On(s.Options)
}

type HelloMsg struct {
	Greeting string
	Name     string
}

func (s *Subscriber) Handler(ctx context.Context, msg *HelloMsg, m *pubsub.Msg) error {
	fmt.Printf("Message received %+v\n\n", m)
	fmt.Printf(msg.Greeting + " " + msg.Name + "\n")
	return nil
}

func NewSubscriber(name, topic string) (*Subscriber) {
	var subscriber = Subscriber{Options: pubsub.HandlerOptions{
		Topic:   topic,
		Name:    "athena-processor-" + name,
		AutoAck: true,
		JSON:    true,
	}}
	subscriber.Options.Handler = subscriber.Handler
	return &subscriber
}

func NewProcessor(params *cmd.DaemonParams) (*Processor, error) {
	return &Processor{Config: params.Config, SalesforceClient: params.SFClient}, nil
}

func (p *Processor) Run() (error) {
	n, err := nats.NewNats("test-cluster")
	if err != nil {
		return err
	}

	pubsub.SetClient(&pubsub.Client{
		ServiceName: "my-new-service",
		Provider:    n,
		Middleware:  defaults.Middleware,
	})

	hostname, err := os.Hostname()
	if err != nil {
		return err
	}

	for _, topic := range p.Config.Processor.SubscribeTo {
		go pubsub.Subscribe(NewSubscriber(hostname, topic))
	}

	fmt.Println("Subscribing to queues..")
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	pubsub.Shutdown()
	return nil
}


