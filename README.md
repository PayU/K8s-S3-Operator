# K8s-S3-Operator
Kubernetes operator which will dynamically or statically provision AWS S3 Bucket storage and access.

### Using the operator on a local cluster

The project uses a [kind](https://kind.sigs.k8s.io/docs/user/quick-start/) cluster for development and test purposes.

Requirements:

* `kind`: [v0.17.0](https://github.com/kubernetes-sigs/kind/releases/tag/v0.17.0)
* `controller-gen` > 0.4
* `kustomize` >= 4.0
* `docker`: latest version
* `golang` >= 1.17

## **Quick-Start**


This script will create kind cluster,
     build image of the controller and deploy it,
     deploy kong ingress controller
     and will run local aws on cluster with ingress ([localstack](https://github.com/localstack/localstack))
```bash
sh ./hack/scripts/runLocalEnv.sh # you might need to run this as sudo if a regular user can't use docker

```

**Set up on non-local env**

Todo
### **Run unit tests**
   ```
   go test ./controllers/... -v
```
### **Run system tests**
The tests run against your local kind cluster and the [localstack](https://github.com/localstack/localstack) service that run on your cluster.

run tests:
```bash
    1. upload local env:
       1.1.  sh ./hack/scripts/runLocalEnv.sh
    2. go test ./tests/systemTest/system_test.go -v # -v flag for log all tests as they are run
```
### **Run integration tests**
The integretion tests test the functionality of integration between deploying/update app to deploying new s3bucket


run tests:

1. deploy local env -> see ([Quick-Start](##Quick-Start))
2. Run
```bash
    sh ./tests/integrationTests/testApp/uploadApp.sh
    go test ./tests/integrationTests/integration_test.go -timeout 120s -v
```

### **Development using Tilt**

The recommended development flow is based on [Tilt](https://tilt.dev/) - it is used for quick iteration on code running in live containers.
Setup based on [official docs](https://docs.tilt.dev/example_go.html) can be found in the Tiltfile.

Prerequisites:

1. Install the Tilt tool
2. Run
```bash
    sh ./hack/scripts/runLocalEnv.sh
``` 
3. Run `tilt up` and go the indicated localhost webpage

```
> tilt up
Tilt started on http://localhost:10350/
v0.22.15, built 2021-10-29

(space) to open the browser
(s) to stream logs (--stream=true)
(t) to open legacy terminal mode (--legacy=true)
(ctrl-c) to exit
```

---
