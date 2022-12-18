CLUSTER_NAME=s3operator-cluster
IMG=apptest:new


docker build -t ${IMG} -f ./appTest.Dockerfile .
kind load docker-image ${IMG} --name ${CLUSTER_NAME}
