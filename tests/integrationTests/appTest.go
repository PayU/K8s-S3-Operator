package main

import (
	"bytes"
	"fmt"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gin-gonic/gin"
)

type bucketObj struct {
	Key  string `json:"key"`
	Body string `json:"body"`
}

func main() {
	router := gin.Default()
	router.GET("/", health)
	router.GET("/bucket/:bucket_name", getBucket)
	router.GET("/bucket/:bucket_name/:obj_key", getObjFromBucket)
	router.POST("/bucket/:bucket_name", addObjToBucket)

	router.Run("localhost:8080")

}
func health(c *gin.Context) {
	fmt.Println("health function")
	c.IndentedJSON(http.StatusOK, `{"health":"good"}`)
}
func getBucket(c *gin.Context) {
	fmt.Println("getBucket function")
	s3Client := s3.New(CreateSession())
	bucketName := c.Params.ByName("bucket_name")
	fmt.Println(bucketName)
	res, err := s3Client.GetBucketLocation(&s3.GetBucketLocationInput{Bucket: &bucketName})
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, err.Error())
	} else {
		c.IndentedJSON(http.StatusOK, res)
	}
}

func getObjFromBucket(c *gin.Context) {
	s3Client := s3.New(CreateSession())
	bucketName := c.Params.ByName("bucket_name")
	obj_key := c.Params.ByName("obj_key")
	res, err := s3Client.GetObject(&s3.GetObjectInput{Bucket: &bucketName, Key: &obj_key})
	if err != nil {
		c.IndentedJSON(http.StatusInternalServerError, err.Error())
	} else {
		c.IndentedJSON(http.StatusOK, res)
	}

}

func addObjToBucket(c *gin.Context) {
	s3Client := s3.New(CreateSession())
	bucketName := c.Params.ByName("bucket_name")
	var input bucketObj
	if err := c.BindJSON(&input); err != nil {
		c.IndentedJSON(http.StatusBadRequest, err)
	} else {
		res, err := s3Client.PutObject(&s3.PutObjectInput{Bucket: &bucketName, Key: &input.Key, Body: bytes.NewReader([]byte(input.Body))})
		if err != nil {
			c.IndentedJSON(http.StatusInternalServerError, err.Error())
		} else {
			c.IndentedJSON(http.StatusOK, res)
		}
	}

}

func CreateSession() *session.Session {
	awsConfig := &aws.Config{
		Region:                        aws.String("eu-central-1"),
		S3ForcePathStyle:              aws.Bool(true),
		Endpoint:                      aws.String("http://localhost:4566/"),
		CredentialsChainVerboseErrors: aws.Bool(true),
		DisableSSL:                    aws.Bool(true),
	}
	awsConfig.HTTPClient = &http.Client{Timeout: time.Duration(5 * time.Second)}
	ses := session.Must(session.NewSession(awsConfig))

	return ses
}
