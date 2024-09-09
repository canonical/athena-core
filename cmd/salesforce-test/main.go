package main

import (
	"fmt"
	"log"
	"strconv"

	"github.com/canonical/athena-core/pkg/common"
	"github.com/canonical/athena-core/pkg/config"
	"gopkg.in/alecthomas/kingpin.v2"
)

var configs = common.StringList(
	kingpin.Flag("config", "Path to the athena configuration file").Default("/etc/athena/main.yaml").Short('c'),
)

var commit string
var allCases = kingpin.Flag("all-cases", "Get all cases").Default("false").Bool()
var allFeedComments = kingpin.Flag("all-feed-comments", "Get all FeedComments").Default("false").Bool()
var allFeedItems = kingpin.Flag("all-feed-items", "Get all FeedItems").Default("false").Bool()
var caseNumber = kingpin.Flag("case-id", "The case ID to query").Default("").String()
var commentVisibility = kingpin.Flag("visibility", "Set the comment visibility {public, private)").Default("private").String()
var getChatter = kingpin.Flag("chatter", "Get all chatter objects of case").Default("false").Bool()
var getComments = kingpin.Flag("comments", "Get all comments of case").Default("false").Bool()
var newChatter = kingpin.Flag("new-chatter", "Add a new chatter comment to the case").Default("").String()
var runQuery = kingpin.Flag("query", "Run query").Default("").String()

func main() {
	log.Printf("Starting version %s", commit)

	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	switch *commentVisibility {
	case "public", "private":
		// All good, do nothing.
	default:
		log.Fatal("Invalid visibility value. Allowed values are 'public' and 'private'.")
	}

	cfg, err := config.NewConfigFromFile(*configs)
	if err != nil {
		panic(err)
	}

	sfClient, err := common.NewSalesforceClient(cfg)
	if err != nil {
		panic(err)
	}

	if *allCases {
		getAllCases(sfClient)
	}

	if *allFeedComments {
		getAllFeedComments(sfClient)
	}

	if *allFeedItems {
		getAllFeedItems(sfClient)
	}

	if len(*caseNumber) > 0 {
		caseId := getCase(sfClient)
		if *getComments {
			getAllCaseComments(caseId, sfClient)
		}

		if *getChatter {
			getAllChatterComments(caseId, sfClient)
		}

		if len(*newChatter) > 0 {
			newChatterComment(sfClient, caseId, *newChatter)
		}
	}

	if len(*runQuery) > 0 {
		getQueryResult(sfClient, *runQuery)
	}
}

func getQueryResult(sfClient common.SalesforceClient, queryString string) {
	log.Printf("Running query: '%s'", queryString)
	records, err := sfClient.Query(queryString)
	if err != nil {
		log.Fatalf("Failed to run query: %v", err)
	}
	if len(records.Records) == 0 {
		log.Fatal("Coudl not find any records")
	}
	for _, record := range records.Records {
		log.Printf("%v", record)
	}
}

func newChatterComment(sfClient common.SalesforceClient, caseId string, comment string) {
	log.Print("Added new chatter comment")
	visibility := ""
	switch *commentVisibility {
	case "public":
		visibility = "AllUsers"
	case "private":
		visibility = "InternalUsers"
	default:
		log.Fatal("Unknown visibility")
	}
	sfClient.SObject("FeedItem").
		Set("ParentId", caseId).
		Set("Body", comment).
		Set("Visibility", visibility).
		Create()
}

func getAllChatterComments(caseId string, sfClient common.SalesforceClient) {
	log.Print("Getting case chatter comments")
	query := fmt.Sprintf("SELECT Id, Body FROM FeedItem WHERE ParentID = '%s'", caseId)
	log.Printf("Running query: '%s'", query)
	records, err := sfClient.Query(query)
	if err != nil {
		log.Fatalf("Failed to get chatter comments: %v", err)
	}
	if len(records.Records) == 0 {
		log.Fatal("Could not find any chatter comments")
	}
	for _, comment := range records.Records {
		log.Printf("%s: %s", comment["Id"], comment["Body"])
	}
}

func getAllCaseComments(caseId string, sfClient common.SalesforceClient) {
	log.Print("Getting case comments")
	query := fmt.Sprintf("SELECT Id, CommentBody FROM CaseComment WHERE ParentId = '%s'", caseId)
	records, err := sfClient.Query(query)
	if err != nil {
		log.Fatalf("Failed to get case comments: %v", err)
	}
	if len(records.Records) == 0 {
		log.Fatal("Could not find any case comments")
	}
	for _, comment := range records.Records {
		log.Printf("%s", comment["CommentBody"])
	}
}

func getCase(sfClient common.SalesforceClient) string {
	caseNumberAsInt, err := strconv.ParseInt(*caseNumber, 10, 64)
	if err != nil {
		log.Fatalf("Failed to parse the case number %s", *caseNumber)
	}
	caseNumberFormatted := fmt.Sprintf("%08d", caseNumberAsInt)

	log.Printf("Searching for case %s", caseNumberFormatted)
	query := fmt.Sprintf("SELECT Id, CaseNumber FROM Case WHERE CaseNumber = '%s'", caseNumberFormatted)
	records, err := sfClient.Query(query)
	if err != nil {
		log.Fatalf("Failed to query Salesforce: %v", err)
	}
	if len(records.Records) > 0 {
		log.Printf("%s: %s", records.Records[0]["Id"], records.Records[0]["CaseNumber"])
	} else {
		log.Fatalf("Case with ID %s does not exist.\n", *caseNumber)
	}
	return fmt.Sprintf("%s", records.Records[0]["Id"])
}

func getAllFeedItems(sfClient common.SalesforceClient) {
	log.Println("All FeedItems:")
	records, err := sfClient.Query("SELECT Id from FeedItem")
	if err != nil {
		log.Fatalln("Failed to query for all FeedItems")
	}
	for _, result := range records.Records {
		log.Printf("%s", result["Id"])
	}
}

func getAllFeedComments(sfClient common.SalesforceClient) {
	log.Println("All FeedComments:")
	records, err := sfClient.Query("SELECT Id from FeedComment")
	if err != nil {
		log.Fatalln("Failed to query for all FeedComments")
	}
	for _, result := range records.Records {
		log.Printf("%s", result["Id"])
	}
}

func getAllCases(sfClient common.SalesforceClient) {
	log.Println("All cases:")
	records, err := sfClient.Query("SELECT Id, CaseNumber from Case")
	if err != nil {
		log.Fatalln("Failed to query for all cases")
	}
	for _, result := range records.Records {
		log.Printf("%s: %s", result["Id"], result["CaseNumber"])
	}
}
