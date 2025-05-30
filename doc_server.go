package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	documentai "cloud.google.com/go/documentai/apiv1"
	"cloud.google.com/go/documentai/apiv1/documentaipb"
	"github.com/gin-gonic/gin"
	"google.golang.org/api/option"
)

const (
	projectID   = "your-project-id"   // Replace with your GCP project ID
	location    = "us"                // Or your region
	processorID = "your-processor-id" // Replace with your Document AI processor ID
	credFile    = ""                  // Path to your service account key file
)

// Create a structured Go struct to hold the business license data
type BusinessLicense struct {
	LicenseID     string `json:"license_id"`
	IssuingOffice string `json:"issuing_office"`
	LicenseeName  string `json:"license_issued_to"`
	BusinessType  string `json:"for_the_business_of"`
	Region        string `json:"region"`
	Ward          string `json:"ward"`
	Street        string `json:"street"`
	BranchType    string `json:"principal_or_branch"`
	AmountPaid    string `json:"amount_of_fee_paid"`
	IssueDate     string `json:"date_of_issue"`
	ExpiryDate    string `json:"expiring_date"`
}

func main() {
	r := gin.Default()

	r.POST("/process", func(c *gin.Context) {
		file, err := c.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "File required"})
			return
		}

		tempFile, err := ioutil.TempFile("", "upload")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create temp file"})
			return
		}
		defer os.Remove(tempFile.Name())

		fileData, err := file.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to open file"})
			return
		}
		defer fileData.Close()

		byteData, _ := ioutil.ReadAll(fileData)
		ioutil.WriteFile(tempFile.Name(), byteData, 0644)

		output, err := processDocument(tempFile.Name())
		if err != nil {
			log.Println("DocAI Error: -------->", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Document processing failed"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"data": output})
	})

	r.Run(":8080")

}

func tryClientConnection(path string) error {
	ctx := context.Background()
	_, err := documentai.NewDocumentProcessorClient(ctx, option.WithCredentialsFile(path))
	if err != nil {
		return fmt.Errorf("❌ Failed to create Document AI client: %v", err)
	}
	fmt.Println("✅ Successfully created Document AI client!")
	return nil
}

func processDocument(filePath string) (*BusinessLicense, error) {

	ctx := context.Background()
	client, err := documentai.NewDocumentProcessorClient(ctx, option.WithCredentialsFile(credFile))
	if err != nil {
		log.Println("Process Doc: ----1---->", err.Error())
		return nil, err
	}
	defer client.Close()

	fileBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Println("Process Doc: ----2---->", err.Error())
		return nil, err
	}

	req := &documentaipb.ProcessRequest{
		Name: fmt.Sprintf("projects/%s/locations/%s/processors/%s", projectID, location, processorID),
		Source: &documentaipb.ProcessRequest_RawDocument{
			RawDocument: &documentaipb.RawDocument{
				Content:  fileBytes,
				MimeType: "image/png",
			},
		},
	}

	// Process the document.
	resp, err := client.ProcessDocument(ctx, req)
	if err != nil {
		log.Printf("Process Doc Error: %v", err)
		return nil, err
	}

	// In your main handler after `ProcessDocument`:
	license := extractBusinessLicense(resp.Document)

	fmt.Println(license)

	return &license, nil
}

// Parse the entities and map them to struct fields
func extractBusinessLicense(doc *documentaipb.Document) BusinessLicense {
	var license BusinessLicense
	for _, entity := range doc.Entities {
		for _, prop := range entity.Properties {
			key := prop.Type
			value := prop.MentionText
			conf := prop.Confidence

			if conf < 0.5 || value == "" {
				continue // skip low confidence or empty values
			}

			switch key {
			case "id":
				license.LicenseID = value
			case "organization":
				if license.IssuingOffice == "" {
					license.IssuingOffice = value
				} else {
					license.BusinessType = value
				}
			case "person":
				if license.LicenseeName == "" {
					license.LicenseeName = value
				} else if license.Region == "" {
					license.Region = value
				} else if license.Ward == "" {
					license.Ward = value
				} else if license.Street == "" {
					license.Street = value
				}
			case "price":
				license.AmountPaid = value
			case "date_time":
				if license.IssueDate == "" {
					license.IssueDate = value
				} else {
					license.ExpiryDate = value
				}
			case "branch_type":
				license.BranchType = value
			}
		}
	}
	return license
}
