CLUSTER_NAME=s3operator-cluster
IMG=apptest:new

cd ./tests/integrationTests/testApp/
docker build -t ${IMG} -f ./appTest.Dockerfile .
kind load docker-image ${IMG} --name ${CLUSTER_NAME}
cd ../
kubectl apply -f ./yamlFiles/deployAuthServer.yaml