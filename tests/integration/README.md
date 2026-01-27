## Running tests locally

First, create the KinD cluster with:

```sh
$ kind create cluster --name kind
```

Then, set it up by running the below command at the repository root:

```sh
$ ./config/setup.sh --local
```

This script first exports the KinD kubeconfig into the `/tmp/kind.kubeconfig` file and then proceeds with building Docker image, loading it into the KinD cluster, installing the latest Flux app and then, at the end, installing the Team Stamper project.

Once the setup is done, run the integration tests with:

```sh
go test -tags=k8srequired -count=1 ./tests/integration -v
```

These tests are configured to look for the `/tmp/kind.kubeconfig` file and use it for creating Kubernetes client.
