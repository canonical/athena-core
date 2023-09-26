package main

import (
	"context"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/canonical/athena-core/pkg/common"
	"github.com/canonical/athena-core/pkg/config"
	"github.com/canonical/athena-core/pkg/monitor"
	"github.com/lileio/pubsub/v2"
	"github.com/lileio/pubsub/v2/providers/nats"
	"github.com/nats-io/stan.go"
	log "github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"
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

	m, err := monitor.NewMonitor(filesClient, sfClient, natsClient, cfg, nil)
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err = m.Run(ctx); err != nil {
		panic(err)
	}
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	cancel()
	pubsub.Shutdown()
}
