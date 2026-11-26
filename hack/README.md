## Getting Started For Developers

- [Lint, Test, Build](#lint-test-build)
  - [Make Targets](#make-targets)
  - [Make Variables](#make-variables)
  - [Execute Standard Environment Functional Tests](#execute-standard-environment-functional-tests)
- [Submit PRs](#submit-prs)
- [Vendoring Dependencies](#vendoring-dependencies)
- [S3-compatible storage setup for velero:](#s3-compatible-storage-setup-for-velero)

### Lint, Test, Build

GnuMake is used to drive a set of scripts that handle linting, testing, compiling, and containerizing.  Executing the scripts directly is not supported at present.

    NOTE: Standard builds require a running Docker daemon!

The standard workflow is performed inside a helper container to normalize the build and test environment for all devs.

`$ make all` executes the full workflow.  For granular control of the workflow, several Make targets are defined:

#### Make Targets

- `all`: compiles the plugin if necessary and builds the image (same as `build-image`)
- `build-all`: compiles the plugin if source files changed
- `build-image`: compiles the plugin if necessary and builds the image
- `build-dirs`: creates output directories
- `push`: pushes image to local registry
- `cluster-push-image`: pushes image to registry of the local cluster
- `local-deploy-velero`: deploys Minio and Velero to the local cluster
- `local-undeploy-velero`: removes Minio and Velero fro the local cluster
- `add-plugin`: adds the plugin to Velero deployment on the local cluster
- `remove-plugin`: removes the plugin from Velero deployment on the local cluster
- `gomod-update`: updates module dependecies
- `clean`: removes output directories and the builder image
- `test`: execute tests
- `test-functional`: build and run functional tests
- `rebuild-functest`: clean and build functional tests
- `clean-test`: clean functional tests
- `cluster-up`: start local k8s cluster and deploy KubeVirt and CDI on it
- `cluster-down`: stop local k8s cluster
- `cluster-sync`: undeploy the plugin, rebuild it and redeploy it

#### Make Variables

Several variables are provided to alter the targets of the above `Makefile` recipes.

These may be passed to a target as `$ make VARIABLE=value target`

- `DOCKER_PREFIX`: (default: kubevirt) Registry and repository prefix of the plugin image
- `IMAGE_NAME`: (default: kubevirt-velero-plugin) Plugin image name
- `DOCKER_TAG`: (default: latest) Plugin image tag
- `ARCH`: (default: linux-amd64) Target OS and architecture, as `<os>-<arch>`
- `WHAT`:  The path from the repository root to a target directory (e.g. `make test WHAT=pkg/importer`)

#### Execute Standard Environment Functional Tests

(This section is a work in progress.)

If using a standard bare-metal/local laptop rhel/kvm environment where nested
virtualization is supported then the standard *kubevirtci framework* can be used.

Environment Variables and Supported Values

| Env Variable       | Default       | Additional Values            |
|--------------------|---------------|------------------------------|
|KUBEVIRT_PROVIDER   | k8s-1.19      | k8s-1.18, k8s-1.20, external |
|KUBEVIRT_STORAGE*   | none          | rook-ceph-default, hpp, nfs, ember_lvm   |
|KUBEVIRT_PROVIDER_EXTRA_ARGS |      |                             |
|NUM_NODES           | 1             | 2-5                         |

To Run Standard Functional Tests
```
 # make cluster-up
 # make cluster-sync
 # make test-functional
```

To run specific functional tests, you can leverage ginkgo command line options as follows:
```
# make TEST_ARGS="--test-args=-ginkgo.focus=<test_suite_name>" test-functional
```
E.g. to run the tests in transport_test.go:
```
# make TEST_ARGS="--test-args=-ginkgo.focus=Transport" test-functional
```

Clean Up
```
 # make cluster-down
```

### Submit PRs

All PRs should originate from forks of kubevirt.io/containerized-data-importer.  Work should not be done directly in the upstream repository.  Open new working branches from master/HEAD of your forked repository and push them to your remote repo.  Then submit PRs of the working branch against the upstream master branch.

### Vendoring Dependencies

This project uses `go modules` as it's dependency manager.  At present, all project dependencies are vendored; using `go mod` is unnecessary in the normal work flow.

`go modules` automatically scans and vendors in dependencies during the build process, you can also manually trigger go modules by running 'make dep-update'.

### S3-compatible storage setup for velero:

Velero is deployed with minio with dummy credentials. See [hack/velero/credentials-velero].
