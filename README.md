# K8s-S3-Operator
Kubernetes operator which will dynamically or statically provision AWS S3 Bucket storage and access.

### Using the operator on a local cluster

The project uses a [kind](https://kind.sigs.k8s.io/docs/user/quick-start/) cluster for development and test purposes.

Requirements:

* `kind`: [v0.17.0](https://github.com/kubernetes-sigs/kind/releases/tag/v0.17.0)
* `controller-gen` > 0.4
* `kustomize` >= 4.0
* `docker`: latest version
* lo

**Quick Start**

This script will create kind cluster, build image and deploy it and will run local aws on cluster ([localstack](https://github.com/localstack/localstack))
```bash
sh ./hack/scripts/runLocalEnv.sh # you might need to run this as sudo if a regular user can't use docker

```

**Set up on non-local env**

Todo

### Development using Tilt

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
