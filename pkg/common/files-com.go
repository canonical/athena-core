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

const DefaultFilesAgeDelta = 10*time.Second

type File struct {
	Created time.Time `gorm:"autoCreateTime"` // Use unix seconds as creating time
	Path    string    `gorm:"primary_key"`
	Md5sum  string    `gorm:"primary_key"`
}

type FilesComClient interface {
	GetFiles(dirs []string) ([]File, error)
	Download(toDownload *File, downloadPath string) (*files_sdk.File, error)
}

type BaseFilesComClient struct{
	FilesComClient
	ApiKey string
}
func (client *BaseFilesComClient) Download(toDownload *File, downloadPath string) (*files_sdk.File, error) {
	files_sdk.APIKey = client.ApiKey
	fcClient := file.Client{}
	////if err := os.MkdirAll(downloadPath, 0755); err != nil {
	//	return nil, err
	//}
	logrus.Infof("downloading file: %s to path: %s", toDownload.Path, downloadPath)
	fileEntry, err := fcClient.DownloadToFile(files_sdk.FileDownloadParams{Path: toDownload.Path}, path.Join(downloadPath, filepath.Base(toDownload.Path)))
	// path.Join(downloadPath, filepath.Base(toDownload.Path)))
	if err != nil {
		return nil, err
	}
	return &fileEntry, nil
}

func (client *BaseFilesComClient) GetFiles(dirs []string) ([]File, error) {
	var files []File
	files_sdk.APIKey = client.ApiKey

	for _, directory := range dirs {
		params := files_sdk.FolderListForParams{Path: directory}
		it, err := folder.ListFor(params)
		if err != nil {
			return nil, err
		}

		for it.Next() {
			path := it.Folder().Path
			sum := it.Folder().Md5
			if it.Folder().Type == "directory" {
				continue
			}

			files = append(files, File{Created: time.Now(), Path: path, Md5sum: sum})
		}
	}
	return files, nil
}

func NewFilesComClient(apiKey string) (FilesComClient, error) {
	return &BaseFilesComClient{ApiKey: apiKey}, nil
}