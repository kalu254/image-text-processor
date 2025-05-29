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
	projectID   = "ai-projects-89ddf"
	location    = "us" // Or your region
	processorID = "your-processor-id"
	credFile    = "service_account.json"
)

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

		c.JSON(http.StatusOK, gin.H{"extracted": output})
	})

	r.Run(":8080")
}

func processDocument(filePath string) (map[string]string, error) {
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

	resp, err := client.ProcessDocument(ctx, req)
	if err != nil {
		log.Println("Process Doc: ----3---->", err.Error())
		return nil, err
	}

	output := map[string]string{}
	for _, entity := range resp.Document.Entities {
		output[entity.Type] = entity.MentionText
	}

	return output, nil
}
