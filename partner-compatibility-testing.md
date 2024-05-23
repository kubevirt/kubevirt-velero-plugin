# Introduction
We have marked several tests with label `PartnerComp`.
Those tests can be used in order to test the compatibility of a backup-restore solution with the kubevirt project.

# Prerequisites
Have a cluster ready to run backup and restore.
If running on Openshift please make sure you have Openshift-Virtualization installed.
If not, Make sure [KubeVirt](https://github.com/kubevirt/kubevirt) and [CDI](https://github.com/kubevirt/containerized-data-importer) are installed in your cluster.

# Requirements
Make a copy of the [template script](cmd/template-script-backup-restore/template-script-backup-restore.sh).
Search for the `TODO` comments.
Adjust and implement the script according to your solution, without changing the API of the script.
You should handle all the flags in order for the tests to call it as needed.
(As an example you can look at [velero backup-restore script](cmd/velero-backup-restore/main.go))

# How to Run
In order to Run the compatibility tests with your script.

## Running locally
In case you can connect to your cluster locally you can run the tests as follows:
`$ export KUBECONFIG=<path/to/kubeconfig>`
(you can also try export KUBECONFIG=~/.kube/config)
`$ export KUBEVIRT_PROVIDER=external`
`$ export BACKUP_SCRIPT_PATH=<path/to/your/script.sh>`
`$ export KVP_STORAGE_CLASS=<desired-storage-class>`
`$ export KVP_BACKUP_NS=<namespace-of-backup-operations>`
`$ make TEST_ARGS="--test-args=-ginkgo.label-filter=PartnerComp" test-functional`

## Running on remote cluster
build the tests binary:
`$ make tests-local`

Copy the tests binary and manifests to remote cluster:
`$ scp _output/tests/tests.test  [user@]your-cluster:tests`
`$ scp tests/manifests  [user@]host:manifests`

In your remote cluster do:
`$ export KVP_STORAGE_CLASS=<desired-storage-class>`
`$ export KVP_BACKUP_NS=<namespace-of-backup-operations>`

Run the tests:
`$ tests/tests.test -ginkgo.v -kubectl-path=<path-to-kubectl> -kubeconfig=<path-to-kubeconfig> -backup-script=<path-to-your-script.sh> -ginkgo.label-filter=PartnerComp`
