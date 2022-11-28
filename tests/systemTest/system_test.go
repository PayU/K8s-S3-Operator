package tests

import (
	"os"
	"testing"

)

func TestMain(m *testing.M){
	// run local env befor 
	
	
	exitVal := m.Run()


	os.Exit(exitVal)
}

func TestCreateBucket(t *testing.T) {
	t.Log("TestCreateBucket")
}
func TestPutBucketData(t *testing.T) {
	t.Log("TestPutBucketData")
}
func TestFetchBucketData(t *testing.T) {
	t.Log("TestFetchBucketData")
}
func TestDeleteBucket(t *testing.T) {
	t.Log("TestDeleteBucket")
}
