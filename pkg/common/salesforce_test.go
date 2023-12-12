package common

import (
	"errors"
	"testing"
)

const validCaseNumber = "123456"
const invalidCaseNumber = "999999"
const authenticationErrorCaseNumber = "111111"

type MockedSalesforceClient struct {
	BaseSalesforceClient
}

func (sf *MockedSalesforceClient) GetCaseByNumber(number string) (*Case, error) {
	if number == validCaseNumber {
		return &Case{CaseNumber: number}, nil
	} else if number == authenticationErrorCaseNumber {
		return nil, ErrAuthentication
	}
	return nil, ErrNoCaseFound{number}
}

func TestGetCaseByNumber(t *testing.T) {
	client := &MockedSalesforceClient{}
	caseNumber := validCaseNumber

	caseDetails, err := client.GetCaseByNumber(caseNumber)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if caseDetails.CaseNumber != caseNumber {
		t.Errorf("Expected case number %s, got %s", caseNumber, caseDetails.CaseNumber)
	}
}

func TestGetCaseByNumberNotFound(t *testing.T) {
	client := &MockedSalesforceClient{}
	caseNumber := invalidCaseNumber
	invalidCaseErr := ErrNoCaseFound{caseNumber}

	caseDetails, err := client.GetCaseByNumber(caseNumber)
	if err == nil {
		t.Errorf("Expected error, got nil")
	}

	if !errors.Is(err, invalidCaseErr) {
		t.Errorf("Expected %v, got %v", invalidCaseErr, err)
	}

	if caseDetails != nil {
		t.Errorf("Expected nil case details, got %+v", caseDetails)
	}
}

func TestGetCaseByNumberAuthentication(t *testing.T) {
	client := &MockedSalesforceClient{}
	caseNumber := authenticationErrorCaseNumber

	caseDetails, err := client.GetCaseByNumber(caseNumber)
	if err == nil {
		t.Errorf("Expected error, got nil")
	}

	if !errors.Is(err, ErrAuthentication) {
		t.Errorf("Expected %v, got %v", ErrAuthentication, err)
	}

	if caseDetails != nil {
		t.Errorf("Expected nil case details, got %+v", caseDetails)
	}
}
