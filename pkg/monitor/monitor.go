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
	Db               *gorm.DB                // Database connection
	Config           *config.Config          // Configuration instance
	FilesClient      common.FilesComClient   // Files.com client
	SalesforceClient common.SalesforceClient // SalesForce client
	Provider         pubsub.Provider         // Messaging provider
	mu               *sync.Mutex             // A mutex
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
		return nil, fmt.Errorf("No processor found for file=%s", filename)
	}
	return processors, nil
}

func (m *Monitor) GetLatestFiles(dirs []string, duration time.Duration) ([]db.File, error) {
	files, err := m.FilesClient.GetFiles(dirs)
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

	for _, file := range files {
		var processors []string

		caseNumber, err := common.GetCaseNumberFromFilename(file.Path)
		if err == nil {
			sfCase, err = m.SalesforceClient.GetCaseByNumber(caseNumber)
			if err != nil {
				// The SalesForce connection possibly died on us. Let's try to
				// revive it and then try again.
				log.Warn("Creating new SF client since current one is failing")
				m.SalesforceClient, err = common.NewSalesforceClient(m.Config)
				if err != nil {
					log.Errorf("Failed to reconnect to salesforce: %s", err)
					panic(err)
				}
				sfCase, err = m.SalesforceClient.GetCaseByNumber(caseNumber)
				if err != nil {
					log.Error(err)
				}
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

func NewMonitor(filesClient common.FilesComClient, salesforceClient common.SalesforceClient, provider pubsub.Provider,
	cfg *config.Config, dbConn *gorm.DB) (*Monitor, error) {
	var err error
	if dbConn == nil {
		dbConn, err = db.GetDBConn(cfg)
		if err != nil {
			return nil, err
		}
	}

	return &Monitor{
		Provider:         provider,
		Db:               dbConn,
		FilesClient:      filesClient,
		SalesforceClient: salesforceClient,
		Config:           cfg,
		mu:               new(sync.Mutex)}, nil
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
			fileEntry, err := m.FilesClient.Download(&file, basePath)
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
