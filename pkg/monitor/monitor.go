package monitor

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/canonical/athena-core/pkg/common"
	"github.com/canonical/athena-core/pkg/common/db"
	"github.com/canonical/athena-core/pkg/config"
	"github.com/lileio/pubsub/v2"
	"github.com/lileio/pubsub/v2/middleware/defaults"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type Monitor struct {
	Config                  *config.Config                 // Configuration instance
	Db                      *gorm.DB                       // Database connection
	FilesComClientFactory   common.FilesComClientFactory   // How to create a new Files.com client
	mu                      *sync.Mutex                    // A mutex
	Provider                pubsub.Provider                // Messaging provider
	SalesforceClientFactory common.SalesforceClientFactory // How to create a new Salesforce client
}

func (m *Monitor) GetMatchingProcessors(filename string, c *common.Case) ([]string, error) {
	var processors []string
	for _, processor := range m.Config.Monitor.ProcessorMap {
		switch processor.Type {
		case "filename":
			{
				if ok, _ := regexp.Match(processor.Regex, []byte(filename)); ok {
					processors = append(processors, processor.Processor)
				}
			}
		case "case":
			{
				if c == nil {
					continue
				}
				if ok, _ := regexp.Match(processor.Regex, []byte(c.CaseNumber)); ok {
					processors = append(processors, processor.Processor)
				}
			}
		default:
			fmt.Printf("No handler found for type=%s", processor.Type)
		}
	}
	if len(processors) <= 0 {
		return nil, fmt.Errorf("no processor found for file=%s", filename)
	}
	return processors, nil
}

func (m *Monitor) GetLatestFiles(dirs []string, duration time.Duration) ([]db.File, error) {
	log.Debugf("Getting files in %v", dirs)
	filesClient, err := m.FilesComClientFactory.NewFilesComClient(m.Config.FilesCom.Key, m.Config.FilesCom.Endpoint)
	if err != nil {
		panic(err)
	}
	files, err := filesClient.GetFiles(dirs)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		m.Db.Where(db.File{Path: file.Path}).FirstOrCreate(&file)
	}

	m.Db.Where("created > ?", time.Now().Add(-duration)).Find(&files)
	return files, nil
}

func (m *Monitor) GetMatchingProcessorByFile(files []db.File) (map[string][]db.File, error) {
	var sfCase = &common.Case{}
	var results = make(map[string][]db.File)

	salesforceClient, err := m.SalesforceClientFactory.NewSalesforceClient(m.Config)
	if err != nil {
		panic(err)
	}

	for _, file := range files {
		var processors []string

		log.Debugf("Analyzing file %s", file.Path)
		caseNumber, err := common.GetCaseNumberFromFilename(file.Path)
		if err == nil {
			sfCase, err = salesforceClient.GetCaseByNumber(caseNumber)
			if err != nil {
				log.Warningf("Failed to get a case from number: '%s'", caseNumber)
			} else {
				log.Debugf("Found customer '%s' for case number %s", sfCase.Customer, caseNumber)
			}
		} else {
			log.Warningf("Failed to identify case from filename '%s': %s", file.Path, err)
		}

		if sfCase != nil {
			processors, err = m.GetMatchingProcessors(file.Path, sfCase)
		} else {
			processors, err = m.GetMatchingProcessors(file.Path, nil)
		}

		for _, processor := range processors {
			results[processor] = append(results[processor], file)
		}

		if err != nil {
			log.Errorf("Failed to identify processor(s) for '%s' (case=%s): %s", file.Path, caseNumber, err)
			continue
		}
	}

	return results, nil
}

func NewMonitor(provider pubsub.Provider, cfg *config.Config, dbConn *gorm.DB,
	salesforceClientFactory common.SalesforceClientFactory,
	filesComClientFactory common.FilesComClientFactory) (*Monitor, error) {
	var err error
	if dbConn == nil {
		dbConn, err = db.GetDBConn(cfg)
		if err != nil {
			return nil, err
		}
	}

	return &Monitor{
		Config:                  cfg,
		Db:                      dbConn,
		FilesComClientFactory:   filesComClientFactory,
		mu:                      new(sync.Mutex),
		Provider:                provider,
		SalesforceClientFactory: salesforceClientFactory,
	}, nil
}

func (m *Monitor) PollNewFiles(ctx *context.Context, duration time.Duration) {
	filesDelta, err := time.ParseDuration(m.Config.Monitor.FilesDelta)
	if err != nil {
		log.Error(err)
		return
	}

	latestFiles, err := m.GetLatestFiles(m.Config.Monitor.Directories, filesDelta)
	if err != nil {
		log.Error(err)
		return
	}

	processors, err := m.GetMatchingProcessorByFile(latestFiles)
	if err != nil {
		log.Error(err)
		return
	}

	filesClient, err := m.FilesComClientFactory.NewFilesComClient(m.Config.FilesCom.Key, m.Config.FilesCom.Endpoint)
	if err != nil {
		panic(err)
	}

	log.Infof("Found %d new files, %d to be processed", len(latestFiles), len(processors))
	for processor, files := range processors {
		for _, file := range files {
			if file.Dispatched {
				log.Infof("File %s already dispatched, skipping", file.Path)
				continue
			}
			log.Infof("Downloading file %s to shared folder", file.Path)
			basePath := m.Config.Monitor.BaseTmpDir
			if basePath == "" {
				basePath = "/tmp"
			}
			if _, err := os.Stat(basePath); os.IsNotExist(err) {
				log.Debugf("Temporary base path '%s' doesn't exist - creating", basePath)
				if err = os.MkdirAll(basePath, 0755); err != nil {
					log.Errorf("Failed to create temporary base path: %s - skipping", err.Error())
					continue
				}
			}
			log.Debugf("Using temporary base path: %s", basePath)
			fileEntry, err := filesClient.Download(&file, basePath)
			if err != nil {
				log.Errorf("Failed to download %s: %s - skipping", file.Path, err)
				continue
			}
			log.Infof("Downloaded %s", fileEntry.Path)

			log.Infof("Sending file: %s to processor: %s", file.Path, processor)
			publishResults := pubsub.PublishJSON(*ctx, processor, file)
			if publishResults.Err != nil {
				file.Dispatched = false
				log.Errorf("Cannot dispatch file: %s to processor, error: %s", file.Path, err)
			} else {
				file.Dispatched = true
				log.Debugf("File: %s -- flagged as dispatched", file.Path)
			}
			m.Db.Save(file)
		}
	}
}
func (m *Monitor) Run(ctx context.Context) error {
	pubsub.SetClient(&pubsub.Client{
		ServiceName: "athena-processor",
		Provider:    m.Provider,
		Middleware:  defaults.Middleware,
	})

	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithCancel(context.Background())
		defer cancel()
	}

	pollEvery, err := time.ParseDuration(m.Config.Monitor.PollEvery)
	if err != nil {
		return err
	}

	go common.RunOnInterval(ctx, m.mu, pollEvery, m.PollNewFiles)
	<-ctx.Done()
	return nil
}
