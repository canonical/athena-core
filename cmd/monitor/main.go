package main

import (
	"context"
	"github.com/lileio/pubsub/v2"
	"github.com/lileio/pubsub/v2/providers/nats"
	"github.com/niedbalski/go-athena/pkg/config"
	"github.com/niedbalski/go-athena/pkg/monitor"
	"github.com/niedbalski/go-athena/pkg/common"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	cfg, err := config.NewConfigFromFile("./example-config.yaml")
	if err != nil {
		panic(err)
	}

	filesClient, err := common.NewFilesComClient(common.GetFilesFromFilesCom, cfg.Monitor.APIKey)
	if err != nil {
		panic(err)
	}

	sfClient , err := common.NewSalesforceClient(cfg)
	if err != nil {
		panic(err)
	}

	n, err := nats.NewNats("test-cluster")
	if err != nil {
		panic(err)
	}

	monitor, err := monitor.NewMonitor(filesClient, sfClient, n, cfg,nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err = monitor.Run(ctx); err != nil {
		panic(err)
	}
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	cancel()
	pubsub.Shutdown()


}
