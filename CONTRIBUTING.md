# Contributing

## Introduction

Let's start with the relationship between several related projects:

* **Kubernetes** is a container orchestration system, and is used to run
  containers on a cluster
* **KubeVirt** is an add-on which is installed on-top of Kubernetes, to be able
  to add basic virtualization functionality to Kubernetes.
* **containerized-data-importer (CDI)** is an add-on which solves the problem of
  populating Kubernetes Persistent Volumes with data.  It was written to be
  general purpose but with the virtualization use case in mind.  Therefore, it
  has a close relationship and special integration with KubeVirt.
* **Velero** is an open source tool to safely backup and restore, perform
  disaster recovery, and migrate Kubernetes cluster resources and persistent volumes.
* **kubevirt-velero-plugin (KVP)** is an add-on to Velero which add support for backing up
  and restoring KubeVirt's and CDI's objects (VMs and DataVolumes).

This short page shall help to get started with the projects and topics
surrounding them.  If you notice a strong similarity with the
[KubeVirt contribution guidelines](https://github.com/kubevirt/kubevirt/blob/master/CONTRIBUTING.md)
it's because we have taken inspiration from their success.

## Contributing to Kubevirt Velero Plugin

### Our workflow

Contributing to KVP should be as simple as possible. Have a question? Want
to discuss something? Want to contribute something? Just open an
[Issue](https://github.com/kubevirt/kubevirt-velero-plugin/issues) or a [Pull
Request](https://github.com/kubevirt/kubevirt-velero-plugin/pulls).
For discussion, we use the [KubeVirt Google Group](https://groups.google.com/forum/#!forum/kubevirt-dev).

If you spot a bug or want to change something pretty simple, just go
ahead and open an Issue and/or a Pull Request, including your changes
at [kubevirt/kubevirt-velero-plugin](https://github.com/kubevirt/kubevirt-velero-plugin).

For bigger changes, please create a tracker Issue, describing what you want to
do. Then either as the first commit in a Pull Request, or as an independent
Pull Request, provide an **informal** design proposal of your intended changes.
The location for such propoals is
[/docs](docs/) in the KVP repository. Make sure that all your Pull Requests link back to the
relevant Issues.

### Getting started

To make yourself comfortable with the code, you might want to work on some
Issues marked with one or more of the following labels
[help wanted](https://github.com/kubevirt/kubevirt-velero-plugin/labels/help%20wanted),
[good first issue](https://github.com/kubevirt/kubevirt-velero-plugin/labels/good%20first%20issue),
or [bug](https://github.com/kubevirt/kubevirt-velero-plugin/labels/kind%2Fbug).
Any help is greatly appreciated.

### Testing

**Untested features do not exist**. To ensure that what we code really works,
relevant flows should be covered via unit tests and functional tests. So when
thinking about a contribution, also think about testability. All tests can be
run local without the need of CI. Have a look at the
[Developer Guide](hack/README.md).

### Getting your code reviewed/merged

Maintainers are here to help you enabling your use-case in a reasonable amount
of time. The maintainers will try to review your code and give you productive
feedback in a reasonable amount of time. However, if you are blocked on a
review, or your Pull Request does not get the attention you think it deserves,
reach out for us via Comments in your Issues, or ping us on
[Slack](https://kubernetes.slack.com/messages/kubevirt-dev).

Maintainers are:

* @tbaransk
* @awels
* @aglitke
* @mhenriks

### PR Checklist

Before your PR can be merged it must meet the following criteria:
* [README.md](README.md) has been updated if core functionality is affected.
* Complex features need standalone documentation in [doc/](doc/).
* Functionality must be fully tested.  Unit test code coverage as reported by
  [Goveralls](https://coveralls.io/github/kubevirt/kubevirt-velero-plugin?branch=master)
  must not decrease unless justification is given (ie. you're adding generated
  code).

## DCO Sign off

All authors to the project retain copyright to their work. However, to ensure
that they are only submitting work that they have rights to, we are requiring
everyone to acknowledge this by signing their work.

Any copyright notices in this repo should specify the authors as "the Velero contributors".

To sign your work, just add a line like this at the end of your commit message:

```
Signed-off-by: Joe Beda <joe@heptio.com>
```

This can easily be done with the `--signoff` option to `git commit`.

By doing this you state that you can certify the following (from https://developercertificate.org/):

```
Developer Certificate of Origin
Version 1.1

Copyright (C) 2004, 2006 The Linux Foundation and its contributors.
1 Letterman Drive
Suite D4700
San Francisco, CA, 94129

Everyone is permitted to copy and distribute verbatim copies of this
license document, but changing it is not allowed.


Developer's Certificate of Origin 1.1

By making a contribution to this project, I certify that:

(a) The contribution was created in whole or in part by me and I
    have the right to submit it under the open source license
    indicated in the file; or

(b) The contribution is based upon previous work that, to the best
    of my knowledge, is covered under an appropriate open source
    license and I have the right under that license to submit that
    work with modifications, whether created in whole or in part
    by me, under the same open source license (unless I am
    permitted to submit under a different license), as indicated
    in the file; or

(c) The contribution was provided directly to me by some other
    person who certified (a), (b) or (c) and I have not modified
    it.

(d) I understand and agree that this project and the contribution
    are public and that a record of the contribution (including all
    personal information I submit with it, including my sign-off) is
    maintained indefinitely and may be redistributed consistent with
    this project or the open source license(s) involved.
```


## Projects & Communities

### [CDI](https://github.com/kubevirt/containerized-data-importer)

* Getting started
  * [Developer Guide](https://github.com/kubevirt/containerized-data-importer/hack/README.md)
  * [Other Documentation](https://github.com/kubevirt/containerized-data-importer/doc/)

### [KubeVirt](https://github.com/kubevirt/)

* Getting started
  * [Developer Guide](docs/getting-started.md)
  * [Demo](https://github.com/kubevirt/demo)

### [Kubernetes](http://kubernetes.io/)

* Getting started
  * [http://kubernetesbyexample.com](http://kubernetesbyexample.com)
  * [Hello Minikube - Kubernetes](https://kubernetes.io/docs/tutorials/stateless-application/hello-minikube/)
  * [User Guide - Kubernetes](https://kubernetes.io/docs/user-guide/)
* Details
  * [Declarative Management of Kubernetes Objects Using Configuration Files - Kubernetes](https://kubernetes.io/docs/concepts/tools/kubectl/object-management-using-declarative-config/)
  * [Kubernetes Architecture](https://github.com/kubernetes/community/blob/master/contributors/design-proposals/architecture/architecture.md)

## Additional Topics

* Golang
  * [Documentation - The Go Programming Language](https://golang.org/doc/)
  * [Getting Started - The Go Programming Language](https://golang.org/doc/install)
* Patterns
  * [Introducing Operators: Putting Operational Knowledge into Software](https://coreos.com/blog/introducing-operators.html)
  * [Microservices](https://martinfowler.com/articles/microservices.html) nice
    content by Martin Fowler
* Testing
  * [Ginkgo - A Golang BDD Testing Framework](https://onsi.github.io/ginkgo/)


