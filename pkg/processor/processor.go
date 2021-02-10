package processor

import (
	"context"
	"fmt"
	"github.com/flosch/pongo2/v4"
	"github.com/go-orm/gorm"
	"github.com/lileio/pubsub/v2"
	"github.com/lileio/pubsub/v2/middleware/defaults"
	"github.com/niedbalski/go-athena/pkg/common"
	"github.com/niedbalski/go-athena/pkg/common/db"
	"github.com/niedbalski/go-athena/pkg/config"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sync"
	"time"
)

type Processor struct {
	Db               *gorm.DB
	Config           *config.Config
	FilesClient      common.FilesComClient
	SalesforceClient common.SalesforceClient
	Provider         pubsub.Provider
	Hostname         string
}

type BaseSubscriber struct {
	Db               *gorm.DB
	Options          pubsub.HandlerOptions
	Reports          map[string]config.Report
	SalesforceClient common.SalesforceClient
	FilesComClient   common.FilesComClient
	Config           *config.Config
	Name             string
}

func (s *BaseSubscriber) Setup(c *pubsub.Client) {
	c.On(s.Options)
}

type ReportToExecute struct {
	Name, Command, BaseDir, ExitCodes, Subscriber string
	File                                          *db.File
	Timeout                                       time.Duration
	Output                                        []byte
}

type ReportRunner struct {
	Reports                   []ReportToExecute
	SalesforceClient          common.SalesforceClient
	FilescomClient            common.FilesComClient
	Name, Subscriber, Basedir string
	Db                        *gorm.DB
}

func RunWithTimeout(report *ReportToExecute) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), report.Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", "-c", report.Command)
	cmd.Dir = report.BaseDir
	//cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: task.Pgid}

	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		//log.Warnf("Collector: %s, timed out after %f secs (cancelled)", report.Name, report.Timeout.Seconds())
		return nil, nil
	}
	return output, err
}

func RunWithoutTimeout(report *ReportToExecute) ([]byte, error) {
	cmd := exec.Command("bash", "-c", report.Command)
	cmd.Dir = report.BaseDir
	return cmd.CombinedOutput()
}

func RunReport(report *ReportToExecute) ([]byte, error) {
	if report.Timeout > 0 {
		return RunWithTimeout(report)
	}
	return RunWithoutTimeout(report)
}

const DefaultReportOutputFormat = "%s.athena-%s-report"

func (runner *ReportRunner) UploadAndSaveReport(report *ReportToExecute, content string) error {
	var file db.File
	filePath := report.File.Path
	result := runner.Db.Where("path = ?", filePath).First(&file)
	if result.Error != nil {
		return fmt.Errorf("cannot find a file with path: %s on the database", filePath)
	}

	uploadedFilePath, err := runner.FilescomClient.Upload(content, fmt.Sprintf(DefaultReportOutputFormat, filePath, report.Name))
	if err != nil {
		return fmt.Errorf("cannot upload file: %s", filePath)
	}

	logrus.Debugf("Uploaded file: %s", uploadedFilePath.DownloadUri)
	caseNumber, err := common.GetCaseNumberByFilename(filePath)
	if err != nil || caseNumber == "" {
		return fmt.Errorf("not found case number on filename: %s", filePath)
	}

	logrus.Infof("Getting case from salesforce number: %s", caseNumber)
	sfCase, err := runner.SalesforceClient.GetCaseByNumber(caseNumber)
	if err != nil {
		return err
	}

	if r := runner.Db.Save(&db.Report{
		Created: time.Now(), CaseID: sfCase.Id, Commented: false, UploadLocation: uploadedFilePath.Path, Name: report.Name, FileID: file.ID, Subscriber: report.Subscriber}); r.Error != nil {
		return err
	}

	logrus.Infof("Saved report name: %s on path:%s - case id: %s", report.Name, uploadedFilePath.Path, sfCase.CaseNumber)
	return nil
}

func (runner *ReportRunner) Run(reportFn func(report *ReportToExecute) ([]byte, error)) error {
	for _, report := range runner.Reports {
		var err error
		var output []byte

		logrus.Debugf("Running report: %s on file: %s", report.Name, report.File.Path)
		output, err = reportFn(&report)
		if err != nil {
			logrus.Error(err)
			continue
		}

		logrus.Debugf("Uploading and saving report:%s for file: %s", report.Name, report.File.Path)
		if err := runner.UploadAndSaveReport(&report, string(output)); err != nil {
			logrus.Errorf("cannot upload and save report: %s - error: %s", report.Name, err)
			continue
		}
	}

	return nil
}

const DefaultExecutionTimeout = "5m"

func renderTemplate(ctx *pongo2.Context, data string) (string, error) {
	tpl, err := pongo2.FromString(data)
	if err != nil {
		return "", err
	}
	out, err := tpl.Execute(*ctx)
	if err != nil {
		return "", err
	}
	return out, nil
}

func NewReportRunner(cfg *config.Config, dbConn *gorm.DB, sf common.SalesforceClient, fc common.FilesComClient, subscriber, name string,
	file *db.File, reports map[string]config.Report) (*ReportRunner, error) {

	var reportRunner ReportRunner
	var command string

	basePath := cfg.Processor.BaseTmpDir
	if basePath == "" {
		basePath = "/tmp"
	}

	logrus.Debugf("Using temporary base path: %s", basePath)
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		logrus.Debugf("Temporary base path: %s doesn't exists, creating", basePath)
		if err = os.MkdirAll(basePath, 0755); err != nil {
			return nil, err
		}
	}

	dir, err := ioutil.TempDir(basePath, "athena-report-"+name)
	if err != nil {
		return nil, err
	}

	fileEntry, err := fc.Download(file, dir)
	if err != nil {
		return nil, err
	}

	reportRunner.Subscriber = subscriber
	reportRunner.Name = name
	reportRunner.Basedir = dir
	reportRunner.Db = dbConn
	reportRunner.SalesforceClient = sf
	reportRunner.FilescomClient = fc

	//TODO: document the template variables
	tplContext := pongo2.Context{
		"basedir":  reportRunner.Basedir,                                           // base dir used to generate reports
		"file":     fileEntry,                                                      // file entry as returned by the files.com api client
		"filepath": path.Join(reportRunner.Basedir, filepath.Base(fileEntry.Path)), // directory where the file lives on
	}

	for name, report := range reports {
		if report.Script != "" {
			fd, err := ioutil.TempFile(reportRunner.Basedir, "run-script-")
			if err != nil {
				return nil, err
			}
			if err = fd.Chmod(0700); err != nil {
				return nil, err
			}

			out, err := renderTemplate(&tplContext, report.Script)
			if err != nil {
				return nil, err
			}

			if _, err = fd.WriteString(out); err != nil {
				return nil, err
			}

			if err = fd.Close(); err != nil {
				return nil, err
			}
			command = fd.Name()
		} else {
			command, err = renderTemplate(&tplContext, report.Command)
			if err != nil {
				return nil, err
			}
		}

		timeout, err := time.ParseDuration(report.Timeout)
		if err != nil {
			timeout, _ = time.ParseDuration(DefaultExecutionTimeout)
		}

		reportToExecute := ReportToExecute{}
		reportToExecute.Timeout = timeout
		reportToExecute.Command = command
		reportToExecute.BaseDir = reportRunner.Basedir
		reportToExecute.ExitCodes = report.ExitCodes
		reportToExecute.Subscriber = reportRunner.Subscriber
		reportToExecute.Name = name
		reportToExecute.File = file
		reportRunner.Reports = append(reportRunner.Reports, reportToExecute)
	}

	return &reportRunner, nil
}

func (runner *ReportRunner) Clean() error {
	logrus.Infof("Removing base directory: %s for report: %s", runner.Basedir, runner.Name)
	return os.RemoveAll(runner.Basedir)
}

func (s *BaseSubscriber) Handler(_ context.Context, file *db.File, msg *pubsub.Msg) error {
	runner, err := NewReportRunner(s.Config, s.Db, s.SalesforceClient, s.FilesComClient, s.Name, s.Options.Topic, file, s.Reports)
	if err != nil {
		logrus.Error(err)
		msg.Ack()
		return err
	}
	if err := runner.Run(RunReport); err != nil {
		logrus.Error(err)
		msg.Ack()
		_ = runner.Clean()
		return err
	}
	msg.Ack()
	return runner.Clean()
}

const defaultHandlerDeadline = 10 * time.Minute

func NewBaseSubscriber(filesClient common.FilesComClient, salesforceClient common.SalesforceClient,
	name, topic string, reports map[string]config.Report, cfg *config.Config, dbConn *gorm.DB) *BaseSubscriber {
	var subscriber = BaseSubscriber{Options: pubsub.HandlerOptions{
		Topic:    topic,
		Name:     "athena-processor-" + name,
		AutoAck:  false,
		JSON:     true,
		Deadline: defaultHandlerDeadline,
	}, Reports: reports}

	subscriber.FilesComClient = filesClient
	subscriber.SalesforceClient = salesforceClient
	subscriber.Options.Handler = subscriber.Handler
	subscriber.Config = cfg
	subscriber.Name = topic
	subscriber.Db = dbConn
	return &subscriber
}

func NewProcessor(filesClient common.FilesComClient, salesforceClient common.SalesforceClient,
	provider pubsub.Provider, cfg *config.Config, dbConn *gorm.DB) (*Processor, error) {
	var err error
	if dbConn == nil {
		dbConn, err = db.GetDBConn(cfg)
		if err != nil {
			return nil, err
		}
	}

	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	return &Processor{
		Hostname:         hostname,
		Provider:         provider,
		FilesClient:      filesClient,
		SalesforceClient: salesforceClient,
		Db:               dbConn,
		Config:           cfg}, nil
}

func (p *Processor) getReportsByTopic(topic string) map[string]config.Report {
	results := make(map[string]config.Report)
	for event, subscriber := range p.Config.Processor.SubscribeTo {
		if event == topic {
			for name, report := range subscriber.Reports {
				results[name] = report
			}
		}
	}
	return results
}

var reportMap map[string]map[string]map[string][]db.Report

func (p *Processor) BatchSalesforceComments(ctx *context.Context, interval time.Duration) {
	var reports []db.Report
	if reportMap == nil {
		reportMap = make(map[string]map[string]map[string][]db.Report)
	}

	logrus.Infof("Running process to send batched comments to salesforce every %s", interval)
	if results := p.Db.Where("created <= ? and commented = ?", time.Now().Add(-interval), false).Find(&reports); results.Error != nil {
		logrus.Error(results.Error)
		return
	}

	if len(reports) <= 0 {
		logrus.Errorf("Not found reports to be processed, skipping")
		return
	}

	logrus.Infof("Found %d reports to be sent to salesforce", len(reports))
	for _, report := range reports {
		if reportMap[report.Subscriber] == nil {
			reportMap[report.Subscriber] = make(map[string]map[string][]db.Report)
		}
		if reportMap[report.Subscriber][report.CaseID] == nil {
			reportMap[report.Subscriber][report.CaseID] = make(map[string][]db.Report)
		}

		if reportMap[report.Subscriber][report.CaseID][report.Name] == nil {
			reportMap[report.Subscriber][report.CaseID][report.Name] = make([]db.Report, 0)
		}

		reportMap[report.Subscriber][report.CaseID][report.Name] = append(reportMap[report.Subscriber][report.CaseID][report.Name], report)
	}

	for subscriberName, caseMap := range reportMap {
		for caseId, reportsByType := range caseMap {
			for _, reports := range reportsByType {
				var tplContext pongo2.Context
				subscriber, ok := p.Config.Processor.SubscribeTo[subscriberName]
				if !ok {
					logrus.Errorf("Not found subscriber for: %s", subscriberName)
					continue
				}

				if !subscriber.SFCommentEnabled {
					logrus.Warnf("Salesforce comments have been disabled, skipping comments")
					continue
				}

				//TODO: document variables
				tplContext = pongo2.Context{
					"processor":  p.Hostname,
					"subscriber": subscriberName,
					"reports":    reports,
				}

				renderedComment, err := renderTemplate(&tplContext, subscriber.SFComment)
				if err != nil {
					logrus.Error(err)
					continue
				}

				comment := p.SalesforceClient.PostComment(caseId, renderedComment, subscriber.SFCommentIsPublic)
				if comment == nil {
					logrus.Errorf("Cannot post comment to case id: %s", caseId)
					continue
				}

				logrus.Infof("Posted comment on case id: %s, %d reports", caseId, len(reports))
				for _, report := range reports {
					report.Commented = true
					p.Db.Save(report)
				}
				reportMap = nil
			}
		}
	}
}

func (p *Processor) Run(ctx context.Context, newSubscriberFn func(filesClient common.FilesComClient,
	salesforceClient common.SalesforceClient, name, topic string, reports map[string]config.Report, cfg *config.Config, dbConn *gorm.DB) pubsub.Subscriber) error {

	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithCancel(context.Background())
		defer cancel()
	}

	pubsub.SetClient(&pubsub.Client{
		ServiceName: "athena-processor",
		Provider:    p.Provider,
		Middleware:  defaults.Middleware,
	})

	for event := range p.Config.Processor.SubscribeTo {
		go pubsub.Subscribe(newSubscriberFn(p.FilesClient, p.SalesforceClient, p.Hostname, event, p.getReportsByTopic(event), p.Config, p.Db))
	}

	interval, err := time.ParseDuration(p.Config.Processor.BatchCommentsEvery)
	if err != nil {
		return err
	}

	go common.RunOnInterval(ctx, &sync.Mutex{}, interval, p.BatchSalesforceComments)

	<-ctx.Done()
	return nil
}
