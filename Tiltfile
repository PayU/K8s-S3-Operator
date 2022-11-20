if k8s_context() != 'kind-s3operator-cluster':
  fail('Expected K8s context to be "kind-s3operator-cluster", found: ' + k8s_context())

compile_cmd = 'CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -o bin/manager main.go'

local_resource(
  's3-operator-compile',
  compile_cmd,
  deps=['./main.go', './api', './controllers']
)


docker_build('controller', '.', 
    dockerfile='./Dockerfile')
k8s_yaml('./config/crd/bases/s3operator.payu.com_s3buckets.yaml')
k8s_yaml('./config/manager/manager.yaml')
k8s_resource(
  new_name='s3-bucket-crd',
  objects=['s3buckets.s3operator.payu.com:CustomResourceDefinition:default'],
)
k8s_resource('controller-manager', port_forwards=[
    port_forward(8080, 8080, "api-server"),
],
)