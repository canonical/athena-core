package common

import (
	"context"
	"fmt"
	"github.com/Ronmi/pastebin"
	"github.com/google/go-github/github"
	"github.com/niedbalski/go-athena/pkg/config"
	"golang.org/x/oauth2"
)

type PastebinClient interface {
	Paste(filenames map[string]string, opts *PastebinOptions) (string, error)
}

const DefaultPastebinExpiration = pastebin.In1Y

type BasePastebinClient struct {
	PastebinClient
	Config *config.Config
}

type PastebinOptions struct {
	Public bool
}

type UbuntuPastebinClient struct {
	BasePastebinClient
}

func (upb *UbuntuPastebinClient) Paste(filenames map[string]string, opts *PastebinOptions) (string, error) {
	return "", nil
}

type GithubGistClient struct {
	BasePastebinClient
}

func (gg *GithubGistClient) Paste(filenames map[string]string, opts *PastebinOptions) (string, error) {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: gg.Config.Pastebin.Key},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)
	client := github.NewClient(tc)

	files := make(map[github.GistFilename]github.GistFile)
	for filename, content := range filenames {
		fname := filename
		fcontent := content
		files[github.GistFilename(filename)] = github.GistFile{
			Filename: &fname,
			Content:  &fcontent,
		}
	}

	ctx := context.Background()
	gist, _, err := client.Gists.Create(ctx, &github.Gist{
		Public: &opts.Public,
		Files:  files,
	})

	if err != nil {
		return "", err
	}

	return *gist.HTMLURL, nil
}

func (pb *BasePastebinClient) Paste(filenames map[string]string, opts *PastebinOptions) (string, error) {
	return "", nil
}

func NewPastebinClient(cfg *config.Config) (PastebinClient, error) {
	switch cfg.Pastebin.Provider {
	case "github":
		return &GithubGistClient{BasePastebinClient{Config: cfg}}, nil
	case "pastebincom":
		return &BasePastebinClient{Config: cfg}, nil
	case "ubuntu":
		return &UbuntuPastebinClient{BasePastebinClient{Config: cfg}}, nil
	}
	return nil, fmt.Errorf("not found implementation for %s", cfg.Pastebin.Provider)
}
