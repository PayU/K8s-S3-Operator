IMG = 'controller:tilt'
load("ext://restart_process", "docker_build_with_restart")

DOCKERFILE = """FROM golang:alpine
WORKDIR /
COPY ./bin/manager /
CMD ["/manager"]
"""

    
def manifests():
    return './bin/controller-gen crd rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases;'


def generate():
    return './bin/controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./...";'


def binary():
    return "CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/manager main.go"

# Deploy CRD
local_resource(
    "CRD",
    manifests() + "kustomize build config/crd | kubectl apply -f -",
    deps=["api"],
    ignore=["*/*/zz_generated.deepcopy.go"],
)

# Deploy manager
watch_file("./config/")
k8s_yaml(local('kustomize build config/default'))

local_resource(
    "Watch & Compile",
    generate() + binary(),
    deps=["controllers", "api", "main.go"],
    ignore=["*/*/zz_generated.deepcopy.go"],
)

docker_build_with_restart(
    IMG,
    ".",
    dockerfile_contents=DOCKERFILE,
    entrypoint=["/manager"],
    only=["./bin/manager"],
    live_update=[
        sync("./bin/manager", "/manager"),
    ],
)
