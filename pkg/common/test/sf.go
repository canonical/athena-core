package test

import (
	files_sdk "github.com/Files-com/files-sdk-go"
	"github.com/niedbalski/go-athena/pkg/common"
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
	{Path: "/uploads/sosreport-testing-1.tar.xz"},
	{Path: "/uploads/sosreport-testing-2.tar.xz"},
	{Path: "/uploads/sosreport-testing-3.tar.xz"},
}

func (fc *TestFilesComClient) GetFiles(dirs []string) ([]common.File, error) {
	return files, nil
}

func (fc *TestFilesComClient) Download(toDownload *common.File, downloadPath string) (*files_sdk.File, error) {
	return nil, nil
}
