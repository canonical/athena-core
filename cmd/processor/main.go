package main

import (
	"context"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/canonical/athena-core/pkg/common"
	"github.com/canonical/athena-core/pkg/config"
	"github.com/canonical/athena-core/pkg/processor"
	"github.com/lileio/pubsub/v2"
	"github.com/lileio/pubsub/v2/providers/nats"
	"github.com/nats-io/stan.go"
	log "github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"
	"gorm.io/gorm"
)

var (
	logLevel = kingpin.Flag("log.level", "Log level: [debug, info, warn, error, fatal]").Default("info").String()
	configs  = common.StringList(kingpin.Flag("config", "Path to the athena configuration file").Default("/etc/athena/main.yaml").Short('c'))
	natsUrl  = kingpin.Flag("nats-url", "URL of the nats service").Default("nats://nats-streaming:4222").String()
	commit   string
)

func init() {
	common.ParseCommandline()
	common.InitLogging(logLevel)
}

func main() {
	cfg, err := config.NewConfigFromFile(*configs)
	if err != nil {
		panic(err)
	}
	log.Infof("Starting athena-processor (%s)", commit)
	log.Debug("Configuration")
	for _, line := range strings.Split(cfg.String(), "\n") {
		log.Debug(line)
	}

	filesClient, err := common.NewFilesComClient(cfg.FilesCom.Key, cfg.FilesCom.Endpoint)
	if err != nil {
		panic(err)
	}

	sfClient, err := common.NewSalesforceClient(cfg)
	if err != nil {
		panic(err)
	}

	natsClient, err := nats.NewNats("test-cluster", stan.NatsURL(*natsUrl))
	if err != nil {
		panic(err)
	}

	p, err := processor.NewProcessor(filesClient, sfClient, natsClient, cfg, nil)
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	if err := p.Run(ctx, func(fc common.FilesComClient, sf common.SalesforceClient, name, topic string,
		reports map[string]config.Report, cfg *config.Config, dbConn *gorm.DB) pubsub.Subscriber {
		log.Infof("Subscribing: %s - to topic: %s", name, topic)
		return processor.NewBaseSubscriber(fc, sf, name, topic, reports, cfg, dbConn)
	}); err != nil {
		panic(err)
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	cancel()
	pubsub.Shutdown()
}
