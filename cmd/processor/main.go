package main

import (
	"context"
	"github.com/lileio/pubsub/v2"
	"github.com/lileio/pubsub/v2/providers/nats"
	"github.com/niedbalski/go-athena/pkg/common"
	"github.com/niedbalski/go-athena/pkg/config"
	"github.com/niedbalski/go-athena/pkg/processor"
	log "github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"syscall"
)

func init() {
	// Log as JSON instead of the default ASCII formatter.
	log.SetFormatter(&log.JSONFormatter{})

	// Output to stdout instead of the default stderr
	// Can be any io.Writer, see below for File example
	log.SetOutput(os.Stdout)

	// Only log the warning severity or above.
	log.SetLevel(log.TraceLevel)
}

func main() {

	cfg, err := config.NewConfigFromFile("./example-config.yaml")
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

	pbClient, err := common.NewPastebinClient(cfg)
	if err != nil {
		panic(err)
	}

	n, err := nats.NewNats("test-cluster")
	if err != nil {
		panic(err)
	}

	p, err := processor.NewProcessor(filesClient, sfClient, pbClient, n, cfg)
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	go p.Run(ctx, func(fc common.FilesComClient, sf common.SalesforceClient, pb common.PastebinClient, name, topic string, reports map[string]config.Report, cfg *config.Config) pubsub.Subscriber {
		log.Infof("Subscribing: %s - to topic: %s", name, topic)
		return processor.NewBaseSubscriber(fc, sf, pb, name, topic, reports, cfg)
	});

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	cancel()
	pubsub.Shutdown()
}