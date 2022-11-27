
IMG = 'controller:tilt'
docker_build(IMG, '.')

def yaml():
    return local('cd config/manager; kustomize edit set image controller=' + IMG + '; cd ../..; kustomize build config/default')

def manifests():
    return 'bin/controller-gen crd:trivialVersions=true rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases;'

def generate():
    return 'bin/controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./...";'

def vetfmt():
    return 'go vet ./...; go fmt ./...'

def binary():
    return 'CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -o bin/manager main.go'

# local(man ifests() + generate())

local_resource('crd', manifests() + 'kustomize build config/crd | kubectl apply -f -', deps=["api"])

k8s_yaml(yaml())

local_resource('recompile', generate() + binary(), deps=['controllers', 'main.go'])
