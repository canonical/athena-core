package main

import (
	"github.com/lileio/pubsub/v2/providers/nats"
	"github.com/niedbalski/go-athena/pkg/config"
	"github.com/niedbalski/go-athena/pkg/monitor"
	"github.com/niedbalski/go-athena/pkg/common"
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
	if err = monitor.Run(); err != nil {
		panic(err)
	}
}
