package test

import (
	files_sdk "github.com/Files-com/files-sdk-go"
	"github.com/niedbalski/go-athena/pkg/common"
	"time"
)

type SalesforceClient struct {
	common.BaseSalesforceClient
}

func (sf *SalesforceClient) GetCaseByNumber(number string) (*common.Case, error) {
	return nil, nil
}

type FilesComClient struct {
	common.BaseFilesComClient
}

var files = []common.File{
	{Path: "/uploads/sosreport-testing-1.tar.xz"},
	{Path: "/uploads/sosreport-testing-2.tar.xz"},
	{Path: "/uploads/sosreport-testing-3.tar.xz"},
}

func (fc *FilesComClient) GetFiles(dirs []string) ([]common.File, error) {
	for i := range files {
		files[i].Created = time.Now()
	}
	return files, nil
}

func (fc *FilesComClient) Download(toDownload *common.File, downloadPath string) (*files_sdk.File, error) {
	return nil, nil
}

type PastebinClient struct {
	common.PastebinClient
}

func (pb *PastebinClient) Paste(filenames map[string]string, opts *common.PastebinOptions) (string, error) {
	return "http://paste.com/123", nil
}
