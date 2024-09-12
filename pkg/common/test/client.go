package test

import (
	"time"

	files_sdk "github.com/Files-com/files-sdk-go"
	"github.com/canonical/athena-core/pkg/common"
	"github.com/canonical/athena-core/pkg/common/db"
	"github.com/canonical/athena-core/pkg/config"
)

type SalesforceClient struct {
	common.BaseSalesforceClient
}

func (sf *SalesforceClient) GetCaseByNumber(number string) (*common.Case, error) {
	return nil, nil
}

type SalesforceClientFactory struct{}

func (sf *SalesforceClientFactory) NewSalesforceClient(config *config.Config) (common.SalesforceClient, error) {
	return &SalesforceClient{}, nil
}

type FilesComClient struct {
	common.BaseFilesComClient
}

type FilesComClientFactory struct{}

func (fc *FilesComClientFactory) NewFilesComClient(apiKey, endpoint string) (common.FilesComClient, error) {
	return &FilesComClient{}, nil
}

var files = []db.File{
	{Path: "/uploads/sosreport-testing-1.tar.xz"},
	{Path: "/uploads/sosreport-testing-2.tar.xz"},
	{Path: "/uploads/sosreport-testing-3.tar.xz"},
}

func (fc *FilesComClient) GetFiles(dirs []string) ([]db.File, error) {
	for i := range files {
		files[i].Created = time.Now()
	}
	return files, nil
}

func (fc *FilesComClient) Download(toDownload *db.File, downloadPath string) (*files_sdk.File, error) {
	return &files_sdk.File{}, nil
}
