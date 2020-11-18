package monitor

import (
	"context"
	"fmt"
	"github.com/go-orm/gorm"
	_ "github.com/go-orm/gorm/dialects/sqlite"
	"github.com/lileio/pubsub/v2"
	"github.com/niedbalski/go-athena/pkg/common"
	"github.com/niedbalski/go-athena/pkg/config"
	"regexp"
	"time"
)


type Monitor struct {
	Db       *gorm.DB
	Config   *config.Config
	FilesClient *common.FilesComClient
	SalesforceClient common.SalesforceClient
	Provider pubsub.Provider
}

func (m *Monitor) GetMatchingProcessors(filename string, c *common.Case) ([]string, error) {
	var processors []string
	for _, processor := range  m.Config.Monitor.ProcessorMap {
		switch processor.Type {
		case "filename": {
			if ok, _ := regexp.Match(string(processor.Regex), []byte(filename)); ok {
				processors = append(processors, processor.Processor)
			}
		}
		case "case": {
			if c == nil {
				continue
			}
			if ok, _ := regexp.Match(string(processor.Regex), []byte(c.CaseNumber)); ok {
				processors = append(processors, processor.Processor)
			}
		}
		default:
			fmt.Printf("Not found handler for %s type", processor.Type)
		}
	}
	if len(processors) <= 0{
		return nil, fmt.Errorf("Not found processors for %s", filename)
	}
	return processors, nil
}

func (m *Monitor) GetLatestFiles(dirs []string, duration time.Duration) ([]common.File, error) {
	files, err := m.FilesClient.GetFiles(m.Config.Monitor.APIKey, dirs)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if err := m.Db.Where(common.File{Path: file.Path, Md5sum: file.Md5sum, Created: time.Now()}).FirstOrCreate(&file).Error; err != nil {
 			return nil, err
		}
	}

	now := time.Now()
	m.Db.Where("created >= ?", now.Add(-duration)).Find(&files)
	return files, nil
}

func (m *Monitor) GetMatchingProcessorByFile(files []common.File) (map[string][]string, error) {
	var sfCase = &common.Case{}
	var results = make(map[string][]string)

	for _, file := range files {
		var processors []string

		caseNumber, err := common.GetCaseNumberByFilename(file.Path)
		if err == nil && caseNumber != "" {
			sfCase, err = m.SalesforceClient.GetCaseByNumber(caseNumber)
			if err != nil {
				fmt.Println(err)
			}
		}

		if sfCase != nil {
			processors, err = m.GetMatchingProcessors(file.Path, sfCase)
		} else {
			processors, err = m.GetMatchingProcessors(file.Path, nil)
		}

		if err != nil {
			fmt.Println(err)
			continue
		}

		results[file.Path] = processors
	}

	return results, nil
}

func NewMonitor(filesClient *common.FilesComClient, salesforceClient common.SalesforceClient, provider pubsub.Provider, cfg *config.Config, db *gorm.DB) (*Monitor, error) {
	if db == nil {
		var err error
		if db, err = gorm.Open("sqlite3", "main.db"); err != nil {
			return nil, err
		}
	}
	return &Monitor{Provider: provider, Db: db, FilesClient: filesClient, SalesforceClient: salesforceClient, Config: cfg}, nil
}

func (m *Monitor) Run() error {
	client := pubsub.Client{
		ServiceName: "test",
		Provider:    m.Provider,
	}

	latestFiles, err := m.GetLatestFiles(m.Config.Monitor.Directories, common.DefaultFilesAgeDelta)
	if err != nil {
		return err
	}

	processors, err := m.GetMatchingProcessorByFile(latestFiles)
	if err != nil {
		return err
	}

	for processor, files := range processors {
		for _, file := range files {
			client.Publish(context.Background(), processor, file, true)
		}
	}

	return nil
}