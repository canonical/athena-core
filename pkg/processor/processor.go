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
}

func (s *BaseSubscriber) Setup(c *pubsub.Client) {
	c.On(s.Options)
}

type ReportToExecute struct {
	Name, Command, BaseDir, ExitCodes string
	File                              *db.File
	Timeout                           time.Duration
	Output                            []byte
}

type ReportRunner struct {
	Reports          []ReportToExecute
	SalesforceClient common.SalesforceClient
	FilescomClient   common.FilesComClient
	Name, Basedir    string
	Db               *gorm.DB
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

func (runner *ReportRunner) UploadAndSave(report *ReportToExecute, content string) error {
	var file db.File

	filePath := report.File.Path
	if result := runner.Db.Where("path = ?", filePath).First(&file); result.Error != nil {
		return fmt.Errorf("cannot find a file with path: %s", filePath)
	}

	uploadedFilePath, err := runner.FilescomClient.Upload(content, filepath.Join(filePath, report.Name))
	if err != nil {
		return fmt.Errorf("cannot upload file: %s", filePath)
	}

	caseNumber, err := common.GetCaseNumberByFilename(filePath)
	if err != nil || caseNumber == "" {
		return fmt.Errorf("not found case number on filename: %s", filePath)
	}

	logrus.Infof("Getting case from salesforce for number: %s", caseNumber)
	sfCase, err := runner.SalesforceClient.GetCaseByNumber(caseNumber)
	if err != nil {
		return err
	}

	if r := runner.Db.Save(&db.Report{
		CaseID: sfCase.Id, Commented: false, UploadLocation: uploadedFilePath.Path, Name: report.Name, FileID: file.ID}); r.Error != nil {
		return err
	}

	logrus.Infof("saved report name: %s on path:%s - case id: %s", report.Name, uploadedFilePath.DownloadUri, sfCase.CaseNumber)
	return nil
}

func (runner *ReportRunner) Run(reportFn func(report *ReportToExecute) ([]byte, error)) (map[string]string, error) {
	var results = make(map[string]string)

	for _, report := range runner.Reports {
		var err error
		var output []byte

		output, err = reportFn(&report)
		if err != nil {
			logrus.Error(err)
			continue
		}
		results[report.Name] = string(output)
		if err := runner.UploadAndSave(&report, string(output)); err != nil {
			logrus.Errorf("cannot upload and save report: %s - error: %s", report.Name, err)
			continue
		}
	}
	return results, nil
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

func NewReportRunner(cfg *config.Config, dbConn *gorm.DB, sf common.SalesforceClient, fc common.FilesComClient, name string, file *db.File, reports map[string]config.Report) (*ReportRunner, error) {
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

	reportRunner.Name = name
	reportRunner.Basedir = dir
	reportRunner.Db = dbConn
	reportRunner.SalesforceClient = sf

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
	runner, err := NewReportRunner(s.Config, s.Db, s.SalesforceClient, s.FilesComClient, s.Options.Topic, file, s.Reports)
	if err != nil {
		logrus.Error(err)
		msg.Ack()
		return err
	}

	logrus.Infof("Running reports on file: %s", file.Path)
	reports, err := runner.Run(RunReport)
	if err != nil {
		logrus.Error(err)
		msg.Ack()
		_ = runner.Clean()
		return err
	}

	if len(reports) <= 0 {
		logrus.Errorf("No reports to process, %d reports", len(reports))
		msg.Ack()
		_ = runner.Clean()
		return nil
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

func (p *Processor) Run(ctx context.Context, newSubscriberFn func(filesClient common.FilesComClient,
	salesforceClient common.SalesforceClient, name, topic string, reports map[string]config.Report, cfg *config.Config, dbConn *gorm.DB) pubsub.Subscriber) {

	pubsub.SetClient(&pubsub.Client{
		ServiceName: "athena-processor",
		Provider:    p.Provider,
		Middleware:  defaults.Middleware,
	})

	for event := range p.Config.Processor.SubscribeTo {
		go pubsub.Subscribe(newSubscriberFn(p.FilesClient, p.SalesforceClient, p.Hostname, event, p.getReportsByTopic(event), p.Config, p.Db))
	}

	<-ctx.Done()
}
