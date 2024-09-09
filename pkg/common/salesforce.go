package common

import (
	"fmt"
	"html"
	"regexp"

	log "github.com/sirupsen/logrus"

	"github.com/canonical/athena-core/pkg/config"
	"github.com/simpleforce/simpleforce"
)

type ErrNoCaseFound struct {
	number string
}

func (e ErrNoCaseFound) Error() string {
	return fmt.Sprintf("no case found in Salesforce with number '%s'", e.number)
}

var ErrAuthentication = simpleforce.ErrAuthentication

type SalesforceClient interface {
	DescribeGlobal() (*simpleforce.SObjectMeta, error)
	GetCaseByNumber(number string) (*Case, error)
	PostChatter(caseId, body string, isPublic bool) *simpleforce.SObject
	PostComment(caseId, body string, isPublic bool) *simpleforce.SObject
	Query(query string) (*simpleforce.QueryResult, error)
	SObject(objectName ...string) *simpleforce.SObject
}

type BaseSalesforceClient struct {
	*simpleforce.Client
}

func NewSalesforceClient(config *config.Config) (SalesforceClient, error) {
	log.Infof("Creating new Salesforce client")
	client := simpleforce.NewClient(config.Salesforce.Endpoint, simpleforce.DefaultClientID, simpleforce.DefaultAPIVersion)
	if err := client.LoginPassword(config.Salesforce.Username, config.Salesforce.Password, config.Salesforce.SecurityToken); err != nil {
		return nil, err
	}
	return &BaseSalesforceClient{client}, nil
}

type Case struct {
	Id, CaseNumber, AccountId, Customer string
}

func (sf *BaseSalesforceClient) GetCaseByNumber(number string) (*Case, error) {
	q := "SELECT Id,CaseNumber,AccountId FROM Case WHERE CaseNumber LIKE '%" + number + "%'"
	result, err := sf.Query(q)
	if err != nil {
		if err == simpleforce.ErrAuthentication {
			return nil, ErrAuthentication
		}
		return nil, err
	}

	for _, record := range result.Records {
		account := sf.SObject("Account").Get(record.StringField("AccountId"))
		if account != nil {
			return &Case{
				Id:         record.StringField("Id"),
				CaseNumber: record.StringField("CaseNumber"),
				AccountId:  record.StringField("AccountId"),
				Customer:   account.StringField("Name"),
			}, nil
		}
	}
	return nil, ErrNoCaseFound{number}
}

func (sf *BaseSalesforceClient) PostComment(caseId, body string, isPublic bool) *simpleforce.SObject {
	return sf.SObject("CaseComment").
		Set("ParentId", caseId).
		Set("CommentBody", html.UnescapeString(body)).
		Set("IsPublished", isPublic).
		Create()
}

func (sf *BaseSalesforceClient) PostChatter(caseId, body string, isPublic bool) *simpleforce.SObject {
	visibility := "InternalUsers"
	if isPublic {
		visibility = "AllUsers"
	}
	return sf.SObject("FeedItem").
		Set("ParentId", caseId).
		Set("Body", body).
		Set("Visibility", visibility).
		Create()
}

func GetCaseNumberFromFilename(filename string) (string, error) {
	regex, err := regexp.Compile(`(\d{6,})`)
	if err != nil {
		return "", err
	}

	for _, candidate := range regex.FindAll([]byte(filename), 1) {
		if len(candidate) <= 8 && len(candidate) > 0 {
			return string(candidate), nil
		}
	}

	return "", fmt.Errorf("failed to identify case number from filename '%s'", filename)
}
