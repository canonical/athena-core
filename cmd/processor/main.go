package main

import (
	"context"
	"github.com/lileio/pubsub/v2"
	"github.com/lileio/pubsub/v2/providers/nats"
	"github.com/nats-io/stan.go"
	"github.com/niedbalski/go-athena/pkg/common"
	"github.com/niedbalski/go-athena/pkg/config"
	"github.com/niedbalski/go-athena/pkg/processor"
	log "github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"
	"os"
	"os/signal"
	"syscall"
)

var (
	logLevel = kingpin.Flag("log.level", "Log level: [debug, info, warn, error, fatal]").Default("info").String()
	configs  = common.StringList(kingpin.Flag("config", "Path to the athena configuration file").Default("/etc/athena/main.yaml").Short('c'))
	natsUrl  = kingpin.Flag("nats-url", "URL of the nats service").Default("nats://nats-streaming:4222").String()
)

func init() {
	common.InitLogging(logLevel)
}

func main() {
	cfg, err := config.NewConfigFromFile(*configs)
	if err != nil {
		panic(err)
	}

	filesClient, err := common.NewFilesComClient(cfg.FilesCom.Key, cfg.FilesCom.Endpoint)
	if err != nil {
		panic(err)
	}

	sfClient, err := common.NewSalesforceClient(cfg)
	if err != nil {
		panic(err)
	}

	pbClient, err := common.NewPastebinClient(cfg)
	if err != nil {
		panic(err)
	}

	natsClient, err := nats.NewNats("test-cluster", stan.NatsURL(*natsUrl))
	if err != nil {
		panic(err)
	}

	p, err := processor.NewProcessor(filesClient, sfClient, pbClient, natsClient, cfg)
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	go p.Run(ctx, func(fc common.FilesComClient, sf common.SalesforceClient, pb common.PastebinClient, name, topic string,
		reports map[string]config.Report, cfg *config.Config) pubsub.Subscriber {
		log.Infof("Subscribing: %s - to topic: %s", name, topic)
		return processor.NewBaseSubscriber(fc, sf, pb, name, topic, reports, cfg)
	})

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	cancel()
	pubsub.Shutdown()
}
