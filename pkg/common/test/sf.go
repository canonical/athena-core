package test

import "github.com/niedbalski/go-athena/pkg/common"

type TestSalesforceClient struct {
	common.BaseSalesforceClient
}

func (sf *TestSalesforceClient) GetCaseByNumber(number string) (*common.Case, error) {
	return nil, nil
}

