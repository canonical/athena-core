package common

import (
	filessdk "github.com/Files-com/files-sdk-go"
	"github.com/Files-com/files-sdk-go/file"
	"github.com/Files-com/files-sdk-go/folder"
	"github.com/niedbalski/go-athena/pkg/common/db"
	"github.com/sirupsen/logrus"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const DefaultFilesAgeDelta = 10 * time.Second

type FilesComClient interface {
	GetFiles(dirs []string) ([]db.File, error)
	Download(toDownload *db.File, downloadPath string) (*filessdk.File, error)
	Upload(contents, destinationPath string) (*filessdk.File, error)
}

type BaseFilesComClient struct {
	FilesComClient
	ApiClient file.Client
}

func (client *BaseFilesComClient) Upload(contents, destinationPath string) (*filessdk.File, error) {
	logrus.Infof("creating new file on path: %s")
	data := strings.NewReader(contents)
	fileEntry, err := client.ApiClient.Upload(data, destinationPath, nil)
	if err != nil {
		return nil, err
	}
	return &fileEntry, nil
}

func (client *BaseFilesComClient) Download(toDownload *db.File, downloadPath string) (*filessdk.File, error) {
	logrus.Infof("downloading file: %s to path: %s", toDownload.Path, downloadPath)
	fileEntry, err := client.ApiClient.DownloadToFile(filessdk.FileDownloadParams{Path: toDownload.Path}, path.Join(downloadPath, filepath.Base(toDownload.Path)))
	if err != nil {
		return nil, err
	}
	return &fileEntry, nil
}

func (client *BaseFilesComClient) GetFiles(dirs []string) ([]db.File, error) {
	var files []db.File

	newClient := folder.Client{Config: client.ApiClient.Config}
	for _, directory := range dirs {
		logrus.Infof("Listing files available on %s", directory)
		params := filessdk.FolderListForParams{Path: directory}
		it, err := newClient.ListFor(params)
		if err != nil {
			return nil, err
		}
		for it.Next() {
			filePath := it.Folder().Path
			if it.Folder().Type == "directory" {
				continue
			}
			files = append(files, db.File{Created: time.Now(), Path: filePath})
		}
	}
	return files, nil
}

func NewFilesComClient(apiKey, endpoint string) (FilesComClient, error) {
	return &BaseFilesComClient{ApiClient: file.Client{Config: filessdk.Config{APIKey: apiKey, Endpoint: endpoint}}}, nil
}
