package tests

import (
	
	"log"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)
func main(){
	AWS_END_POINT := "http://172.19.0.4:31566"
	BUCKET_NAME := "s3OperatorSystemTest"
	ses := session.Must(session.NewSession(&aws.Config{
		Region:                        aws.String(endpoints.UsEast1RegionID),
		S3ForcePathStyle:              aws.Bool(true),
		Endpoint:                      aws.String(AWS_END_POINT),
		CredentialsChainVerboseErrors: aws.Bool(true),
		DisableSSL:                    aws.Bool(true),
	},
	))
	s3Client := s3.New(ses)

	bucketTag,err := s3Client.GetBucketTagging(&s3.GetBucketTaggingInput{Bucket: &BUCKET_NAME})
	if err != nil{//check bucket exists and have "createdBy" tag
		log.Fatalln("err in GetBucketTagging",err)
	}else{
		log.Println("bucket tag",bucketTag.TagSet)
	}
	_,err = s3Client.PutObject(&s3.PutObjectInput{Body:aws.ReadSeekCloser(strings.NewReader("test app")),Key: aws.String("TestApp"),Bucket: &BUCKET_NAME })
	if err != nil{//put object to bucket 
		log.Fatalln("err in PutObject",err)
	}else{
		log.Println("succeded to put object")
	}

	res, err := s3Client.GetObject(&s3.GetObjectInput{Bucket:&BUCKET_NAME,Key: aws.String("TestApp") })
	if err != nil{//Get object to bucket 
		log.Fatalln("err in GetObject",err)
	}else{
		log.Println("succeded to Get object",res)
	}

}