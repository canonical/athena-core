package processor

import (
    "context"
    "fmt"
    "github.com/canonical/athena-core/pkg/common"
    "github.com/canonical/athena-core/pkg/common/db"
    "github.com/canonical/athena-core/pkg/config"
    "github.com/flosch/pongo2/v4"
    "github.com/go-orm/gorm"
    "github.com/lileio/pubsub/v2"
    "github.com/lileio/pubsub/v2/middleware/defaults"
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
    Name, BaseDir, Subscriber string
    File                      *db.File
    Scripts                   map[string]string
    Timeout                   time.Duration
    Output                    []byte
}

type ReportRunner struct {
    Config                    *config.Config
    Reports                   []ReportToExecute
    SalesforceClient          common.SalesforceClient
    FilescomClient            common.FilesComClient
    Name, Subscriber, Basedir string
    Db                        *gorm.DB
}

func RunWithTimeout(baseDir string, timeout time.Duration, command string) ([]byte, error) {
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()
    cmd := exec.CommandContext(ctx, "bash", "-c", command)
    cmd.Dir = baseDir
    //cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: task.Pgid}

    output, err := cmd.CombinedOutput()
    if ctx.Err() == context.DeadlineExceeded {
        //log.Warnf("Collector: %s, timed out after %f secs (cancelled)", report.Name, report.Timeout.Seconds())
        return nil, nil
    }
    return output, err
}

func RunWithoutTimeout(baseDir string, command string) ([]byte, error) {
    cmd := exec.Command("bash", "-c", command)
    cmd.Dir = baseDir
    return cmd.CombinedOutput()
}

func RunReport(report *ReportToExecute) (map[string][]byte, error) {
    var output = make(map[string][]byte)

    for scriptName, script := range report.Scripts {
        logrus.Debugf("Running script:%s for report: %s", scriptName, report.Name)
        var ret []byte
        var err error
        if report.Timeout > 0 {
            ret, err = RunWithTimeout(report.BaseDir, report.Timeout, script)
            if err != nil {
                logrus.Error(err)
                return nil, err
            }
        } else {
            ret, err = RunWithoutTimeout(report.BaseDir, script)
            if err != nil {
                logrus.Error(err)
                return nil, err
            }
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

    logrus.Debugf("fetching files on path: %s", filePath)
    result := runner.Db.Where("path = ?", filePath).First(&file)
    if result.Error != nil {
        return fmt.Errorf("cannot find a file with path: %s on the database", filePath)
    }

    if runner.Config.Processor.ReportsUploadPath == "" {
        uploadPath = filePath
    } else {
        uploadPath = path.Join(runner.Config.Processor.ReportsUploadPath, filepath.Base(filePath))
    }

    logrus.Infof("Getting case from salesforce number: %s", caseNumber)
    sfCase, err := runner.SalesforceClient.GetCaseByNumber(caseNumber)
    if err != nil {
        logrus.Warn("Creating new SF client since current one is failing")
        client, client_err := common.NewSalesforceClient(runner.Config)
        if client_err != nil {
            logrus.Warn("Failed to reconnect to salesforce")
            return err
        }
        runner.SalesforceClient = client
        sfCase, err = runner.SalesforceClient.GetCaseByNumber(caseNumber)
    }
    if err != nil {
        return err
    }

    logrus.Debugf("Got case %s from salesforce", sfCase)
    var newReport = new(db.Report)

    newReport.Created = time.Now()
    newReport.CaseID = sfCase.Id
    newReport.FilePath = file.Path
    newReport.Name = report.Name
    newReport.FileID = file.ID
    newReport.Subscriber = report.Subscriber

    logrus.Debugf("Uploading script outputs")
    for scriptName, output := range scriptOutputs {
        uploadedFilePath, err := runner.FilescomClient.Upload(string(output), fmt.Sprintf(DefaultReportOutputFormat, uploadPath, report.Name, scriptName))
        if err != nil {
            return fmt.Errorf("cannot upload file: %s", filePath)
        }

        logrus.Debugf("Uploaded file: %s", uploadedFilePath.Path)
        newReport.Scripts = append(newReport.Scripts, db.Script{
            Output:         string(output),
            Name:           scriptName,
            UploadLocation: uploadedFilePath.Path,
        })
    }

    if r := runner.Db.Create(newReport); r.Error != nil {
        logrus.Errorf("error creating new report: %s", newReport.FilePath)
        return err
    }

    if r := runner.Db.Save(newReport); r.Error != nil {
        logrus.Errorf("error creating new report: %s", newReport.FilePath)
        return err
    }

    logrus.Infof("Saved report name: %s for case id: %s", report.Name, sfCase.CaseNumber)
    return nil
}

func (runner *ReportRunner) Run(reportFn func(report *ReportToExecute) (map[string][]byte, error)) error {
    for _, report := range runner.Reports {
        var err error

        caseNumber, err := common.GetCaseNumberFromFilename(report.File.Path)
        if err != nil {
            logrus.Error(err)
            continue
        }

        logrus.Debugf("Running report: %s on file: %s", report.Name, report.File.Path)
        scriptOutputs, err := reportFn(&report)
        if err != nil {
            logrus.Error(err)
            continue
        }

        logrus.Debugf("Uploading and saving report:%s script outputs: %d - for file: %s", report.Name, len(scriptOutputs), report.File.Path)
        if err := runner.UploadAndSaveReport(&report, caseNumber, scriptOutputs); err != nil {
            logrus.Errorf("Failed to upload and save report: %s - error: %s", report.Name, err)
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

    reportRunner.Config = cfg
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

    var scripts = make(map[string]string)

    for reportName, report := range reports {
        logrus.Debugf("running %d scripts for report: %s", len(report.Scripts), reportName)
        for scriptName, script := range report.Scripts {
            if script.Run == "" {
                logrus.Errorf("not provided script to run for: %s", scriptName)
                continue
            }
            fd, err := ioutil.TempFile(reportRunner.Basedir, "run-script-")
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
        reportToExecute.Timeout = timeout
        reportToExecute.BaseDir = reportRunner.Basedir
        reportToExecute.Subscriber = reportRunner.Subscriber
        reportToExecute.Name = reportName
        reportToExecute.File = file
        reportToExecute.Scripts = scripts
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
    if results := p.Db.Preload("Scripts").Where("created <= ? and commented = ?", time.Now().Add(-interval), false).Find(&reports); results.Error != nil {
        logrus.Errorf("error getting batched comments: %s", results.Error)
        return
    }

    if len(reports) <= 0 {
        logrus.Warnf("Not found reports to be processed, skipping")
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
