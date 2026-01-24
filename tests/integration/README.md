## Running tests locally

First, create the KinD cluster with:

```sh
$ kind create cluster --name kind
```

Then, set it up by running the below command at the repository root:

```sh
$ ./config/setup.sh --local
```

This script builds the Docker image, load it into KinD cluster, then installs Flux and Team Stamper project.

Once done, run the integration tests with:

```sh
go test -tags=k8srequired -count=1 ./tests/integration -v
```
