package test

import (
	files_sdk "github.com/Files-com/files-sdk-go"
	"github.com/niedbalski/go-athena/pkg/common"
	"time"
)

type TestSalesforceClient struct {
	common.BaseSalesforceClient
}

func (sf *TestSalesforceClient) GetCaseByNumber(number string) (*common.Case, error) {
	return nil, nil
}

type TestFilesComClient struct {
	common.BaseFilesComClient
}

var files = []common.File{
	{Path: "/uploads/sosreport-testing-1.tar.xz", Md5sum: "aabb"},
	{Path: "/uploads/sosreport-testing-2.tar.xz", Md5sum: "bbcc"},
	{Path: "/uploads/sosreport-testing-3.tar.xz", Md5sum: "ccdd"},
}

func (fc *TestFilesComClient) GetFiles(dirs []string) ([]common.File, error) {
	for i := range files {
		files[i].Created = time.Now()
	}
	return files, nil
}

func (fc *TestFilesComClient) Download(toDownload *common.File, downloadPath string) (*files_sdk.File, error) {
	return nil, nil
}

type TestPastebinClient struct {
	common.PastebinClient
}

func (pb *TestPastebinClient) Paste(filenames map[string]string, opts *common.PastebinOptions) (string, error) {
	return "http://paste.com/123", nil
}
