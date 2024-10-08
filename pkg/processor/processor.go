package processor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/canonical/athena-core/pkg/common"
	"github.com/canonical/athena-core/pkg/common/db"
	"github.com/canonical/athena-core/pkg/config"
	"github.com/flosch/pongo2/v4"
	"github.com/lileio/pubsub/v2"
	"github.com/lileio/pubsub/v2/middleware/defaults"
	"github.com/simpleforce/simpleforce"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type Processor struct {
	Config                  *config.Config
	Db                      *gorm.DB
	FilesComClientFactory   common.FilesComClientFactory
	Hostname                string
	Provider                pubsub.Provider
	SalesforceClientFactory common.SalesforceClientFactory
}

type BaseSubscriber struct {
	Config                  *config.Config
	Db                      *gorm.DB
	FilesComClientFactory   common.FilesComClientFactory
	Name                    string
	Options                 pubsub.HandlerOptions
	Reports                 map[string]config.Report
	SalesforceClientFactory common.SalesforceClientFactory
}

func (s *BaseSubscriber) Setup(c *pubsub.Client) {
	c.On(s.Options)
}

type ReportToExecute struct {
	File                                *db.File
	Name, BaseDir, Subscriber, FileName string
	Output                              []byte
	Scripts                             map[string]string
	Timeout                             time.Duration
}

type ReportRunner struct {
	Config                    *config.Config
	Db                        *gorm.DB
	FilesComClientFactory     common.FilesComClientFactory
	Name, Subscriber, Basedir string
	Reports                   []ReportToExecute
	SalesforceClientFactory   common.SalesforceClientFactory
}

func RunWithTimeout(baseDir string, timeout time.Duration, command string) ([]byte, error) {
	log.Debugf("Running script with %s timeout in %s", timeout, baseDir)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", "-c", command)
	cmd.Dir = baseDir
	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return nil, nil
	}
	return output, err
}

func RunWithoutTimeout(baseDir string, command string) ([]byte, error) {
	log.Debugf("Running script without timeout in %s", baseDir)
	cmd := exec.Command("bash", "-c", command)
	cmd.Dir = baseDir
	return cmd.CombinedOutput()
}

func RunReport(report *ReportToExecute) (map[string][]byte, error) {
	var output = make(map[string][]byte)

	for scriptName, script := range report.Scripts {
		log.Debugf("Running script '%s' on sosreport '%s'", scriptName, filepath.Base(report.FileName))
		var ret []byte
		var err error
		if report.Timeout > 0 {
			ret, err = RunWithTimeout(report.BaseDir, report.Timeout, script)
		} else {
			ret, err = RunWithoutTimeout(report.BaseDir, script)
		}
		log.Debugf("Script '%s' on '%s' completed", scriptName, filepath.Base(report.FileName))
		if err != nil {
			log.Errorf("Error occurred (test) while running script: %s", err)
			for _, line := range strings.Split(string(ret), "\n") {
				log.Error(line)
			}
			return nil, err
		}
		output[scriptName] = ret
	}

	return output, nil
}

const DefaultReportOutputFormat = "%s.athena-%s.%s"

func (runner *ReportRunner) UploadAndSaveReport(report *ReportToExecute, caseNumber string, scriptOutputs map[string][]byte) error {
	var file db.File
	var uploadPath string
	filePath := report.File.Path

	log.Debugf("Fetching files for path '%s' from db", filePath)
	result := runner.Db.Where("path = ?", filePath).First(&file)
	if result.Error != nil {
		return fmt.Errorf("file not found with path '%s' in database", filePath)
	}

	log.Infof("Fetching case with number '%s' from Salesforce", caseNumber)
	salesforceClient, err := runner.SalesforceClientFactory.NewSalesforceClient(runner.Config)
	if err != nil {
		log.Errorf("failed to get Salesforce connection: %s", err)
		return err
	}
	sfCase, err := salesforceClient.GetCaseByNumber(caseNumber)
	if err != nil {
		log.Error(err)
		return err
	}

	log.Debugf("Case %s successfully fetched from Salesforce", sfCase)
	var newReport = new(db.Report)

	newReport.CaseID = sfCase.Id
	newReport.Created = time.Now()
	newReport.FileID = file.ID
	newReport.FileName = filepath.Base(file.Path)
	newReport.FilePath = file.Path
	newReport.Name = report.Name
	newReport.Subscriber = report.Subscriber

	if runner.Config.Processor.ReportsUploadPath == "" {
		uploadPath = filePath
	} else {
		uploadPath = path.Join(runner.Config.Processor.ReportsUploadPath, newReport.FileName)
	}

	filesComClient, err := runner.FilesComClientFactory.NewFilesComClient(runner.Config.FilesCom.Key, runner.Config.FilesCom.Endpoint)
	if err != nil {
		log.Errorf("failed to get new file.com client: %s", err)
		return err
	}
	log.Debugf("Uploading script output(s) to files.com")
	for scriptName, output := range scriptOutputs {
		dst_fname := fmt.Sprintf(DefaultReportOutputFormat, uploadPath, report.Name, scriptName)
		log.Debugf("Uploading script output %s", dst_fname)
		uploadedFilePath, err := filesComClient.Upload(string(output), dst_fname)
		if err != nil {
			return fmt.Errorf("failed to upload file '%s': %s", dst_fname, err.Error())
		}

		log.Debugf("Successfully uploaded file '%s'", uploadedFilePath.Path)
		script_result := db.Script{
			Output:         string(output),
			Name:           scriptName,
			UploadLocation: uploadedFilePath.Path,
		}
		newReport.Scripts = append(newReport.Scripts, script_result)
	}

	if r := runner.Db.Create(newReport); r.Error != nil {
		log.Errorf("Failed to create new report for '%s' in db", newReport.FilePath)
		return err
	}

	if r := runner.Db.Save(newReport); r.Error != nil {
		log.Errorf("Failed to save new report for '%s' in db", newReport.FilePath)
		return err
	}

	log.Infof("Saved report '%s' in db for case id '%s'", report.Name, sfCase.CaseNumber)
	return nil
}

func (runner *ReportRunner) Run(reportFn func(report *ReportToExecute) (map[string][]byte, error)) error {
	for _, report := range runner.Reports {
		var err error

		caseNumber, err := common.GetCaseNumberFromFilename(report.File.Path)
		if err != nil {
			log.Info(err)
			continue
		}

		log.Debugf("Running '%s' on '%s'", report.Name, report.File.Path)
		scriptOutputs, err := reportFn(&report)
		if err != nil {
			log.Error(err)
			continue
		}

		log.Debugf("Uploading and saving results of running '%s' on '%s' (count=%d)", report.Name, report.FileName, len(scriptOutputs))
		if err := runner.UploadAndSaveReport(&report, caseNumber, scriptOutputs); err != nil {
			log.Errorf("Failed to upload and save output of '%s': %s", report.Name, err)
			continue
		}
	}

	return nil
}

const DefaultExecutionTimeout = "0s"

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

func NewReportRunner(cfg *config.Config, dbConn *gorm.DB,
	salesforceClientFactory common.SalesforceClientFactory,
	filesComClientFactory common.FilesComClientFactory,
	subscriber, name string,
	file *db.File, reports map[string]config.Report) (*ReportRunner, error) {

	var reportRunner ReportRunner

	basePath := cfg.Processor.BaseTmpDir
	if basePath == "" {
		basePath = "/tmp"
	}

	log.Debugf("Using temporary base path: %s", basePath)
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		log.Debugf("Temporary base path '%s' doesn't exist - creating", basePath)
		if err = os.MkdirAll(basePath, 0755); err != nil {
			return nil, err
		}
	}

	dir, err := os.MkdirTemp(basePath, "athena-report-"+name)
	if err != nil {
		return nil, err
	}
	log.Debugf("Created basedir %s", dir)

	err = os.Rename(filepath.Join(basePath, filepath.Base(file.Path)), filepath.Join(dir, filepath.Base(file.Path)))
	if err != nil {
		return nil, err
	}
	log.Debugf("Moved file to %s", dir)

	reportRunner.Basedir = dir
	reportRunner.Config = cfg
	reportRunner.Db = dbConn
	reportRunner.FilesComClientFactory = filesComClientFactory
	reportRunner.Name = name
	reportRunner.SalesforceClientFactory = salesforceClientFactory
	reportRunner.Subscriber = subscriber

	//TODO: document the template variables
	tplContext := pongo2.Context{
		"basedir":  reportRunner.Basedir,                                      // base dir used to generate reports
		"file":     filepath.Base(file.Path),                                  // file entry as returned by the files.com api client
		"filepath": path.Join(reportRunner.Basedir, filepath.Base(file.Path)), // directory where the file lives on
	}

	var scripts = make(map[string]string)

	for reportName, report := range reports {
		log.Debugf("running %d '%s' script(s)", len(report.Scripts), reportName)
		for scriptName, script := range report.Scripts {
			if script.Run == "" {
				log.Errorf("No script provided to run on '%s'", scriptName)
				continue
			}
			fd, err := os.CreateTemp(reportRunner.Basedir, "run-script-")
			if err != nil {
				return nil, err
			}
			if err = fd.Chmod(0700); err != nil {
				return nil, err
			}

			out, err := renderTemplate(&tplContext, script.Run)
			if err != nil {
				return nil, err
			}

			if _, err = fd.WriteString(out); err != nil {
				return nil, err
			}

			if err = fd.Close(); err != nil {
				return nil, err
			}

			scripts[scriptName] = fd.Name()
		}

		timeout, err := time.ParseDuration(report.Timeout)
		if err != nil {
			timeout, _ = time.ParseDuration(DefaultExecutionTimeout)
		}

		reportToExecute := ReportToExecute{}
		reportToExecute.BaseDir = reportRunner.Basedir
		reportToExecute.File = file
		reportToExecute.FileName = file.Path
		reportToExecute.Name = reportName
		reportToExecute.Scripts = scripts
		reportToExecute.Subscriber = reportRunner.Subscriber
		reportToExecute.Timeout = timeout
		reportRunner.Reports = append(reportRunner.Reports, reportToExecute)
	}

	return &reportRunner, nil
}

func (runner *ReportRunner) Clean() error {
	if runner.Config.Processor.KeepProcessingOutput {
		log.Infof("Keeping base direcotry %s for report %s", runner.Basedir, runner.Name)
		return nil
	}
	log.Infof("Removing base directory: %s for report: %s", runner.Basedir, runner.Name)
	return os.RemoveAll(runner.Basedir)
}

func (s *BaseSubscriber) Handler(_ context.Context, file *db.File, msg *pubsub.Msg) error {
	runner, err := NewReportRunner(s.Config, s.Db, s.SalesforceClientFactory, s.FilesComClientFactory, s.Name, s.Options.Topic, file, s.Reports)
	if err != nil {
		log.Errorf("Failed to get new runner: %s", err)
		msg.Ack()
		return err
	}
	if err := runner.Run(RunReport); err != nil {
		log.Errorf("Runner failed: %s", err)
		msg.Ack()
		_ = runner.Clean()
		return err
	}
	msg.Ack()
	return runner.Clean()
}

const defaultHandlerDeadline = 10 * time.Minute

func NewBaseSubscriber(
	filesComClientFactory common.FilesComClientFactory, salesforceClientFactory common.SalesforceClientFactory,
	name, topic string, reports map[string]config.Report, cfg *config.Config, dbConn *gorm.DB) *BaseSubscriber {
	var subscriber = BaseSubscriber{
		Options: pubsub.HandlerOptions{
			Topic:    topic,
			Name:     "athena-processor-" + name,
			AutoAck:  false,
			JSON:     true,
			Deadline: defaultHandlerDeadline,
		},
		Reports: reports,
	}

	subscriber.Config = cfg
	subscriber.Db = dbConn
	subscriber.FilesComClientFactory = filesComClientFactory
	subscriber.Name = topic
	subscriber.Options.Handler = subscriber.Handler
	subscriber.SalesforceClientFactory = salesforceClientFactory
	return &subscriber
}

func NewProcessor(
	filesComClientFactory common.FilesComClientFactory, salesforceClientFactory common.SalesforceClientFactory,
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
		Config:                  cfg,
		Db:                      dbConn,
		FilesComClientFactory:   filesComClientFactory,
		Hostname:                hostname,
		Provider:                provider,
		SalesforceClientFactory: salesforceClientFactory,
	}, nil
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

// splitComment splits the given comment into several pieces at most
// maxLength characters long.
// The function returns the resulting slice.
func splitComment(comment string, maxLength int) []string {
	// Check length and split across MaxCommentLength character
	// boundaries.
	if len(comment) > maxLength {
		log.Infof("Comment exceeds %d characters; splitting", maxLength)
		var commentChunks []string = []string{}
		commentLines := strings.Split(strings.TrimRight(comment, "\n "), "\n")
		chunk := []string{}
		chunkLength := 0
		for _, line := range commentLines {
			if chunkLength+len(line) < maxLength || len(chunk) == 0 {
				chunkLength += len(line) + 1 // Account for newline
				chunk = append(chunk, line)
			} else {
				commentChunks = append(commentChunks, strings.Join(chunk, "\n"))
				chunkLength = len(line) + 1 // Account for newline
				chunk = []string{line}
			}
		}
		commentChunks = append(commentChunks, strings.Join(chunk, "\n"))
		log.Infof("Comment was split into %d chunks", len(commentChunks))
		return commentChunks
	} else {
		return []string{comment}
	}
}

func (p *Processor) BatchSalesforceComments(ctx *context.Context, interval time.Duration) {
	var reports []db.Report
	if reportMap == nil {
		reportMap = make(map[string]map[string]map[string][]db.Report)
	}

	log.Infof("Running process to send batched comments to salesforce every %s", interval)
	if results := p.Db.Preload("Scripts").Where("created <= ? and commented = ?", time.Now().Add(-interval), false).Find(&reports); results.Error != nil {
		log.Errorf("Error getting batched comments: %s", results.Error)
		return
	}

	if len(reports) <= 0 {
		log.Info("No reports found to be processed - skipping")
		return
	}

	log.Infof("Found %d reports to be sent to Salesforce", len(reports))
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

	salesforceClient, err := p.SalesforceClientFactory.NewSalesforceClient(p.Config)
	if err != nil {
		log.Errorf("failed to get Salesforce client: %s", err)
		return
	}
	for subscriberName, caseMap := range reportMap {
		for caseId, reportsByType := range caseMap {
			for _, reports := range reportsByType {
				var tplContext pongo2.Context
				subscriber, ok := p.Config.Processor.SubscribeTo[subscriberName]
				if !ok {
					log.Errorf("No subscription found for subscriber '%s'", subscriberName)
					continue
				}

				if !subscriber.SFCommentEnabled {
					log.Warnf("Salesforce comments have been disabled, skipping comments")
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
					log.Error(err)
					continue
				}

				log.Infof("Processing comment for case %s", caseId)
				commentChunks := splitComment(renderedComment, p.Config.Salesforce.MaxCommentLength)

				wasPosted := true
				for i, chunk := range commentChunks {
					var chunkHeader string
					if len(commentChunks) > 1 {
						chunkHeader = fmt.Sprintf("Split comment %d of %d\n\n", i+1, len(commentChunks))
					}
					var comment *simpleforce.SObject
					if p.Config.Salesforce.EnableChatter {
						comment = salesforceClient.PostChatter(caseId,
							chunkHeader+chunk, subscriber.SFCommentIsPublic)
					} else {
						comment = salesforceClient.PostComment(caseId,
							chunkHeader+chunk, subscriber.SFCommentIsPublic)
					}
					if comment == nil {
						log.Errorf("Failed to post comment to case id: %s", caseId)
						wasPosted = false
						continue
					}
				}

				if wasPosted {
					log.Infof("Successfully posted comment on case %s for %d reports", caseId, len(reports))
					for _, report := range reports {
						report.Commented = true
						p.Db.Save(report)
					}
					reportMap = nil
				} else {
					log.Errorf("Could not post comment to case id: %s", caseId)
				}
			}
		}
	}
}

func (p *Processor) Run(ctx context.Context, newSubscriberFn func(
	filesComClientFactory common.FilesComClientFactory,
	salesforceClientFactory common.SalesforceClientFactory,
	name, topic string, reports map[string]config.Report,
	cfg *config.Config, dbConn *gorm.DB) pubsub.Subscriber) error {

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
		go pubsub.Subscribe(newSubscriberFn(p.FilesComClientFactory, p.SalesforceClientFactory,
			p.Hostname, event, p.getReportsByTopic(event), p.Config, p.Db))
	}

	interval, err := time.ParseDuration(p.Config.Processor.BatchCommentsEvery)
	if err != nil {
		return err
	}

	go common.RunOnInterval(ctx, &sync.Mutex{}, interval, p.BatchSalesforceComments)

	<-ctx.Done()
	return nil
}
