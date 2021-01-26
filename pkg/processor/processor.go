package processor

import (
	"context"
	"github.com/flosch/pongo2/v4"
	"github.com/lileio/pubsub/v2"
	"github.com/lileio/pubsub/v2/middleware/defaults"
	"github.com/niedbalski/go-athena/pkg/common"
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
	Config           *config.Config
	FilesClient      common.FilesComClient
	SalesforceClient common.SalesforceClient
	PastebinClient   common.PastebinClient
	Provider         pubsub.Provider
	Hostname         string
}

type BaseSubscriber struct {
	Options          pubsub.HandlerOptions
	Reports          map[string]config.Report
	SalesforceClient common.SalesforceClient
	FilesComClient   common.FilesComClient
	PastebinClient   common.PastebinClient
	Config           *config.Config
}

func (s *BaseSubscriber) Setup(c *pubsub.Client) {
	c.On(s.Options)
}

type ReportToExecute struct {
	Name, Command, BaseDir, ExitCodes string
	File                              *common.File
	Timeout                           time.Duration
	Output                            []byte
}

type ReportRunner struct {
	Reports       []ReportToExecute
	Name, Basedir string
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
	//cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: task.Pgid}
	return cmd.CombinedOutput()
}

func RunReport(report *ReportToExecute) ([]byte, error) {
	if report.Timeout > 0 {
		return RunWithTimeout(report)
	}
	return RunWithoutTimeout(report)
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
	}
	return results, nil
}

const DefaultExecutionTimeout = "2m"

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

func NewReportRunner(sf common.SalesforceClient, fc common.FilesComClient, name string, file *common.File, reports map[string]config.Report) (*ReportRunner, error) {
	var reportRunner ReportRunner
	var command string

	dir, err := ioutil.TempDir("/tmp", "athena-report-"+name)
	if err != nil {
		return nil, err
	}

	fileEntry, err := fc.Download(file, dir)
	if err != nil {
		return nil, err
	}

	reportRunner.Name = name
	reportRunner.Basedir = dir

	//TODO: document the template variables
	tplContext := pongo2.Context{
		"basedir":  reportRunner.Basedir,                                          // base dir used to generate reports
		"file":     fileEntry,                                                     // file entry as returned by the files.com api client
		"filedir":  path.Join(reportRunner.Basedir, filepath.Dir(fileEntry.Path)), //directory where the file lives on
		"fullpath": path.Join(reportRunner.Basedir, fileEntry.Path),               // full path to the file
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

func (s *BaseSubscriber) Handler(_ context.Context, file *common.File, _ *pubsub.Msg) error {
	runner, err := NewReportRunner(s.SalesforceClient, s.FilesComClient, s.Options.Topic, file, s.Reports)
	if err != nil {
		logrus.Error(err)
		return err
	}

	logrus.Infof("Running reports on file: %s", file.Path)
	reports, err := runner.Run(RunReport)
	if err != nil {
		return err
	}

	logrus.Infof("Running pastebin client for: %d reports", len(reports))
	url, err := s.PastebinClient.Paste(reports, &common.PastebinOptions{Public: false})
	if err != nil {
		return err
	}

	var tplContext pongo2.Context

	//TODO: move this into a function
	for _, event := range s.Config.Processor.SubscribeTo {
		if event.Topic == s.Options.Topic {
			//TODO: document the template variables
			tplContext = pongo2.Context{
				"processor":    s.Options.Name,
				"filename":     file.Path,
				"pastebin_url": url,
				"reports":      reports,
			}
			caseNumber, err := common.GetCaseNumberByFilename(file.Path)
			if err != nil || caseNumber == "" {
				logrus.Errorf("Not found case number on filename: %s", file.Path)
				continue
			}

			logrus.Infof("Getting case from salesforce for number: %s", caseNumber)
			sfCase, err := s.SalesforceClient.GetCaseByNumber(caseNumber)
			if err != nil {
				logrus.Error(err)
				continue
			}
			renderedComment, err := renderTemplate(&tplContext, event.SFComment)
			if err != nil {
				logrus.Error(err)
				continue
			}
			logrus.Debugf("Posting case comment (id: %s), body: %s", caseNumber, renderedComment)
			comment := s.SalesforceClient.PostComment(sfCase.Id, renderedComment, true)
			if comment == nil {
				logrus.Errorf("Cannot post comment to case id: %s", sfCase.Id)
				continue
			}
			logrus.Infof("Posted comment on case id: %s", caseNumber)
		}
	}

	return nil
}

func NewBaseSubscriber(filesClient common.FilesComClient, salesforceClient common.SalesforceClient, pastebinClient common.PastebinClient, name, topic string, reports map[string]config.Report, cfg *config.Config) *BaseSubscriber {
	var subscriber = BaseSubscriber{Options: pubsub.HandlerOptions{
		Topic:   topic,
		Name:    "athena-processor-" + name,
		AutoAck: true,
		JSON:    true,
	}, Reports: reports}

	subscriber.FilesComClient = filesClient
	subscriber.SalesforceClient = salesforceClient
	subscriber.PastebinClient = pastebinClient
	subscriber.Options.Handler = subscriber.Handler
	subscriber.Config = cfg
	return &subscriber
}

func NewProcessor(filesClient common.FilesComClient, salesforceClient common.SalesforceClient, pastebinClient common.PastebinClient,
	provider pubsub.Provider, cfg *config.Config) (*Processor, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}

	return &Processor{Hostname: hostname, Provider: provider, FilesClient: filesClient, SalesforceClient: salesforceClient, PastebinClient: pastebinClient, Config: cfg}, nil
}

func (p *Processor) getReportsByTopic(topic string) map[string]config.Report {

	results := make(map[string]config.Report)
	for _, c := range p.Config.Processor.SubscribeTo {
		if c.Topic == topic {
			for name, report := range c.Reports {
				results[name] = report
			}
		}
	}
	return results
}

func (p *Processor) Run(ctx context.Context, newSubscriberFn func(filesClient common.FilesComClient,
	salesforceClient common.SalesforceClient, pb common.PastebinClient,
	name, topic string, reports map[string]config.Report, cfg *config.Config) pubsub.Subscriber) {

	pubsub.SetClient(&pubsub.Client{
		ServiceName: "athena-processor",
		Provider:    p.Provider,
		Middleware:  defaults.Middleware,
	})

	for _, event := range p.Config.Processor.SubscribeTo {
		go pubsub.Subscribe(newSubscriberFn(p.FilesClient, p.SalesforceClient, p.PastebinClient,
			p.Hostname, event.Topic, p.getReportsByTopic(event.Topic), p.Config))
	}

	<-ctx.Done()
}
