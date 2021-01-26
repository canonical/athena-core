package common

import (
	"fmt"
	"github.com/niedbalski/go-athena/pkg/config"
	"github.com/simpleforce/simpleforce"
	"regexp"
)

type SalesforceClient interface {
	GetCaseByNumber(number string) (*Case, error)
	PostComment(caseId, body string, isPublic bool) *simpleforce.SObject
}

type BaseSalesforceClient struct {
	*simpleforce.Client
}

func NewSalesforceClient(config *config.Config) (SalesforceClient, error) {
	client := simpleforce.NewClient(config.Salesforce.Endpoint, simpleforce.DefaultClientID, simpleforce.DefaultAPIVersion)
	if err := client.LoginPassword(config.Salesforce.Username, config.Salesforce.Password, config.Salesforce.SecurityToken); err != nil {
		return nil, err
	}
	return &BaseSalesforceClient{client}, nil
}

type Case struct {
	Id, CaseNumber, AccountId, Customer string
}

func (sf *BaseSalesforceClient) PostComment(caseId, body string, isPublic bool) *simpleforce.SObject {
	return sf.SObject("CaseComment").
		Set("ParentId", caseId).
		Set("CommentBody", body).
		Set("IsPublished", isPublic).
		Create()
}

func (sf *BaseSalesforceClient) GetCaseByNumber(number string) (*Case, error) {
	q := "SELECT Id,CaseNumber,AccountId FROM Case WHERE CaseNumber LIKE '%" + number + "%'"
	result, err := sf.Query(q)
	if err != nil {
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
	return nil, fmt.Errorf("Not found case with number: %s", number)
}

func GetCaseNumberByFilename(filename string) (string, error) {
	regex, err := regexp.Compile(`(\d{6,})`)
	if err != nil {
		return "", err
	}

	for _, candidate := range regex.FindAll([]byte(filename), 1) {
		if len(candidate) <= 8 {
			return string(candidate), nil
		}
	}

	return "", fmt.Errorf("Not found case number on: %s", filename)
}
