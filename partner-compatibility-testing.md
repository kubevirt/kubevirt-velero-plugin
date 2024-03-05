# Introduction
We have marked several tests with label `PartnerComp`.
Those tests can be used in order to test the compatibility of a backup-restore solution with the kubevirt project.

# Prerequisites
Have a cluster ready to run backup and restore. Make sure [KubeVirt](https://github.com/kubevirt/kubevirt) and [CDI](https://github.com/kubevirt/containerized-data-importer) are installed.

# Requirements
Make a copy of the [template script](cmd/backup-restore-script-template/main.go).
Search for the `TODO` comments adjust and implement the script according to
your solution. The API of the script needs to stay the same as of the template,
you should handle all the flags in order for the tests to call it as needed and pass.
(you can look at the default [velero backup-restore script](cmd/velero-backup-restore/main.go) as an example of a working script)

# How to Run
In order to Run the compatibility tests with your script:
`$ export KUBECONFIG=<path/to/kubeconfig>`
`$ export KUBEVIRT_PROVIDER=external`
`$ export BACKUP_SCRIPT_PATH=<path/to/thescript/main.go>`
`$ make TEST_ARGS="--test-args=-ginkgo.label-filter=PartnerComp" test-functional`
