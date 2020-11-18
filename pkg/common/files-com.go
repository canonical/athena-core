package common

import (
	files_sdk "github.com/Files-com/files-sdk-go"
	"github.com/Files-com/files-sdk-go/folder"
	"time"
)

const DefaultFilesAgeDelta = 10*time.Second

type File struct {
	Created time.Time `gorm:"autoCreateTime"` // Use unix seconds as creating time
	Path    string    `gorm:"primary_key"`
	Md5sum  string    `gorm:"primary_key"`
}

type FilesGetter func(apikey string, dirs []string) ([]File, error)

type FilesComClient struct{
	ApiKey string
	GetFiles FilesGetter
}

func NewFilesComClient(fg FilesGetter, apiKey string) (*FilesComClient, error) {
	return &FilesComClient{GetFiles: fg, ApiKey: apiKey}, nil
}

func GetFilesFromFilesCom(apikey string, dirs []string) ([]File, error)	{
	var files []File
	files_sdk.APIKey = apikey

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