package main

import (
	"github.com/niedbalski/go-athena/cmd"
	"github.com/niedbalski/go-athena/pkg/config"
	"github.com/niedbalski/go-athena/pkg/processor"
	"github.com/niedbalski/go-athena/pkg/common"
)

func main() {
	cfg, err := config.NewConfigFromFile("./example-config.yaml")
	if err != nil {
		panic(err)
	}

	sfclient, err := common.NewSalesforceClient(cfg)
	if err != nil {
		panic(err)
	}

	processor, err := processor.NewProcessor(&cmd.DaemonParams{SFClient: sfclient, Config: cfg})
	if err != nil {
		panic(err)
	}

	if err := processor.Run(); err != nil {
		panic(err)
	}
}