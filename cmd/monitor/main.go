package main

import (
	"context"
	"github.com/lileio/pubsub/v2"
	"github.com/lileio/pubsub/v2/providers/nats"
	"github.com/nats-io/stan.go"
	"github.com/niedbalski/go-athena/pkg/common"
	"github.com/niedbalski/go-athena/pkg/config"
	"github.com/niedbalski/go-athena/pkg/monitor"
	log "github.com/sirupsen/logrus"
	"gopkg.in/alecthomas/kingpin.v2"
	"os"
	"os/signal"
	"syscall"
)

var (
	logLevel   = kingpin.Flag("log.level", "Log level: [debug, info, warn, error, fatal]").Default("info").String()
	configPath = kingpin.Flag("config", "Path to the athena configuration file").Default("/etc/athena/main.yaml").Short('c').String()
	natsUrl    = kingpin.Flag("nats-url", "URL of the nats service").Default("nats://nats-streaming:4222").String()
)

func init() {
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	// Log as JSON instead of the default ASCII formatter.
	log.SetFormatter(&log.JSONFormatter{})

	// Output to stdout instead of the default stderr
	// Can be any io.Writer, see below for File example
	log.SetOutput(os.Stdout)

	// Only log the warning severity or above.
	level, err := log.ParseLevel(*logLevel)
	if err != nil {
		log.Errorf("Cannot init set logger level: %s", err)
		os.Exit(-1)
	}

	log.SetLevel(level)
}

func main() {

	cfg, err := config.NewConfigFromFile(*configPath)
	if err != nil {
		panic(err)
	}

	filesClient, err := common.NewFilesComClient(cfg.Monitor.APIKey)
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
	if err = m.Run(ctx, common.DefaultFilesAgeDelta); err != nil {
		panic(err)
	}
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	cancel()
	pubsub.Shutdown()
}
