CLUSTER_NAME=s3operator-cluster
IMG ?= controller:tilt
NAMESPACE = k8s-s3-operator-system


make create-local-cluster
current_context=$(kubectl config current-context)
if [ "$current_context" = "kind-$CLUSTER_NAME" ]; then
    echo "building image"
    make docker-build
    echo "load image to kind"
    make kind-load-controller
    echo "deploy operator"
    make deploy

    echo "run local aws loacalstack on cluster"
    make run-local-aws-on-cluster
else
  echo "Please set the current cluster context to kind-$CLUSTER_NAME and re-run the install script"
fi

# increasing inotify max users in order to aviod 'kind' too many open files errors
# more info can be found here: https://github.com/kubernetes-sigs/kind/issues/2586
KIND_DOCKER_IDS=$(docker ps -a -q)
KIND_DOCKER_IDS_ARRAY=($KIND_DOCKER_IDS)

for dockerID in "${KIND_DOCKER_IDS_ARRAY[@]}"
do
  :
  export dockerName=$(docker inspect $dockerID | jq .[0].Name)
  if [[ "$dockerName" == *${IMG}* ]]; then
      echo "increase inotify max users for docker: $dockerName"
      docker exec -t $dockerID bash -c "echo 'fs.inotify.max_user_watches=1048576' >> /etc/sysctl.conf" 
      docker exec -t $dockerID bash -c "echo 'fs.inotify.max_user_instances=512' >> /etc/sysctl.conf"
      docker exec -i $dockerID bash -c "sysctl -p /etc/sysctl.conf"
  fi

done
