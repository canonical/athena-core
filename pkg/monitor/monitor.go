package monitor

import (
	"context"
	"fmt"
	"github.com/canonical/athena-core/pkg/common"
	"github.com/canonical/athena-core/pkg/common/db"
	"github.com/canonical/athena-core/pkg/config"
	"github.com/go-orm/gorm"
	_ "github.com/go-orm/gorm/dialects/sqlite"
	"github.com/lileio/pubsub/v2"
	"github.com/lileio/pubsub/v2/middleware/defaults"
	log "github.com/sirupsen/logrus"
	"regexp"
	"sync"
	"time"
)

type Monitor struct {
	Db               *gorm.DB
	Config           *config.Config
	FilesClient      common.FilesComClient
	SalesforceClient common.SalesforceClient
	Provider         pubsub.Provider
	mu               *sync.Mutex
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
			fmt.Printf("Not found handler for %s type", processor.Type)
		}
	}
	if len(processors) <= 0 {
		return nil, fmt.Errorf("Not found processors for %s", filename)
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
				log.Error(err)
			}
		} else {
			log.Error(err)
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
			log.Error(err)
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
		Config:           cfg, mu: new(sync.Mutex)}, nil
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
			log.Infof("Sending file: %s to processor: %s", file.Path, processor)
			publishResults := pubsub.PublishJSON(*ctx, processor, file)
			if publishResults.Err != nil {
				file.Dispatched = false
				log.Errorf("Cannot dispatch file: %s to processor, error: %s", file.Path, err)
			} else {
				file.Dispatched = true
				log.Debugf("file: %s -- flagged as dispatched", file.Path)
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
