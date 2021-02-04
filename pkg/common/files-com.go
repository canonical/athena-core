package common

import (
	files_sdk "github.com/Files-com/files-sdk-go"
	file "github.com/Files-com/files-sdk-go/file"
	"github.com/Files-com/files-sdk-go/folder"
	"github.com/sirupsen/logrus"
	"path"
	"path/filepath"
	"time"
)

const DefaultFilesAgeDelta = 10 * time.Second

type File struct {
	Created time.Time `gorm:"autoCreateTime"` // Use unix seconds as creating time
	Path    string    `gorm:"primary_key"`
}

type FilesComClient interface {
	GetFiles(dirs []string) ([]File, error)
	Download(toDownload *File, downloadPath string) (*files_sdk.File, error)
}

type BaseFilesComClient struct {
	FilesComClient
	ApiClient file.Client
}

func (client *BaseFilesComClient) Download(toDownload *File, downloadPath string) (*files_sdk.File, error) {
	logrus.Infof("downloading file: %s to path: %s", toDownload.Path, downloadPath)
	fileEntry, err := client.ApiClient.DownloadToFile(files_sdk.FileDownloadParams{Path: toDownload.Path}, path.Join(downloadPath, filepath.Base(toDownload.Path)))
	if err != nil {
		return nil, err
	}
	return &fileEntry, nil
}

func (client *BaseFilesComClient) GetFiles(dirs []string) ([]File, error) {
	var files []File

	fclient := folder.Client{Config: client.ApiClient.Config}
	for _, directory := range dirs {
		logrus.Infof("Listing files available on %s", directory)
		params := files_sdk.FolderListForParams{Path: directory}
		it, err := fclient.ListFor(params)
		if err != nil {
			return nil, err
		}
		for it.Next() {
			path := it.Folder().Path
			//sum := it.Folder().Md5
			if it.Folder().Type == "directory" {
				continue
			}
			files = append(files, File{Created: time.Now(), Path: path})
		}
	}
	return files, nil
}

func NewFilesComClient(apiKey, endpoint string) (FilesComClient, error) {
	return &BaseFilesComClient{ApiClient: file.Client{Config: files_sdk.Config{APIKey: apiKey, Endpoint: endpoint}}}, nil
}
