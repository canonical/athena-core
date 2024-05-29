package common

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"time"

	filessdk "github.com/Files-com/files-sdk-go"
	"github.com/Files-com/files-sdk-go/file"
	"github.com/Files-com/files-sdk-go/folder"
	"github.com/canonical/athena-core/pkg/common/db"
	log "github.com/sirupsen/logrus"
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
	log.Infof("Uploading to '%s'", destinationPath)
	tmpfile, err := os.CreateTemp("", "upload")
	if err != nil {
		return nil, err
	}
	err = os.WriteFile(tmpfile.Name(), []byte(contents), 0644)
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpfile.Name())
	status, err := client.ApiClient.UploadFile(context.Background(), &file.UploadParams{Source: tmpfile.Name(), Destination: destinationPath})
	if err != nil {
		return nil, err
	}
	return &status.Files()[0], nil
}

func (client *BaseFilesComClient) Download(toDownload *db.File, downloadPath string) (*filessdk.File, error) {
	log.Infof("Downloading '%s' to '%s'", toDownload.Path, downloadPath)
	fileEntry, err := client.ApiClient.DownloadToFile(context.Background(), filessdk.FileDownloadParams{Path: toDownload.Path}, path.Join(downloadPath, filepath.Base(toDownload.Path)))
	if err != nil {
		return nil, err
	}
	return &fileEntry, nil
}

func (client *BaseFilesComClient) GetFiles(dirs []string) ([]db.File, error) {
	var files []db.File
	newClient := folder.Client{Config: client.ApiClient.Config}
	for _, directory := range dirs {
		log.Infof("Listing files available on %s", directory)
		it, err := newClient.ListFor(context.Background(), filessdk.FolderListForParams{Path: directory})
		if err != nil {
			return nil, err
		}
		for it.Next() {
			filePath := it.Folder().Path
			if it.Folder().Type == "directory" {
				continue
			}
			log.Debugf("Found file with path: %s", filePath)
			files = append(files, db.File{Created: time.Now(), Path: filePath})
		}
	}
	log.Infof("Found %d files on the target directories", len(files))
	return files, nil
}

func NewFilesComClient(apiKey, endpoint string) (FilesComClient, error) {
	log.Infof("Creating new files.com client")
	return &BaseFilesComClient{ApiClient: file.Client{Config: filessdk.Config{APIKey: apiKey, Endpoint: endpoint}}}, nil
}
