package main

import (
	"fmt"
	"os"

	files_sdk "github.com/Files-com/files-sdk-go"
	"github.com/Files-com/files-sdk-go/folder"
	"github.com/canonical/athena-core/pkg/common"
	"github.com/canonical/athena-core/pkg/config"
	"gopkg.in/alecthomas/kingpin.v2"
)

var configs = common.StringList(
	kingpin.Flag("config", "Path to the athena configuration file").Default("/etc/athena/main.yaml").Short('c'),
)

var path = kingpin.Flag("path", "Path to check").Default("/").String()

func main() {
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	cfg, err := config.NewConfigFromFile(*configs)
	if err != nil {
		panic(err)
	}

	filesConfig := files_sdk.Config{
		APIKey:   cfg.FilesCom.Key,
		Endpoint: cfg.FilesCom.Endpoint,
	}

	folderClient := folder.Client{Config: filesConfig}
	folderIterator, err := folderClient.ListFor(files_sdk.FolderListForParams{Path: *path})
	if err != nil {
		fmt.Printf("Error reading folder: %s", err)
		os.Exit(1)
	}
	for folderIterator.Next() {
		folder := folderIterator.Folder()
		fmt.Println(folder.Path)
	}
}
