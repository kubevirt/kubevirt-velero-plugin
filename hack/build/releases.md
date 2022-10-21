# Version and Release

* [Overview](#overview)
* [Version Scheme](#version-scheme)
* [Releasing a New Version](#releasing-a-new-version)
    * [Promoting the Release](#promoting-the-release)
    * [PROW CI](#prow-ci)
# Overview

## Version Scheme

The **kubevirt-velero-plugin** has similar release process to the kubevirt and cdi. 
It adheres to the [semantic version definitions](https://semver.org/) format of vMAJOR.MINOR.PATCH.  These are defined as follows:

- Major - Non-backwards compatible, API contract changes.  Incrementing a Major version means the consumer will have to 
make changes to the way they interact with the Velero Backup/Restore API.  Another option is the need to follow Major version 
change in the velero project. When these changes occur, the Major version will be incremented at the end of the sprint 
instead of the Minor Version.

- Minor - End of Sprint release. Encapsulates non-API-breaking changes within the current Major version.  
The current Sprint cycle is 2 weeks long, producing in bug fixes and feature additions. Publishing a Minor version 
at the end of the cycle allows consumers to immediately access the end product of that Sprint's goals. Issues or bugs 
can be reported and addressed in the following Sprint.

- Patch - mid-Sprint release for fixing blocker bugs. In the case that a bug is blocking kubevirt-velero-plugin consumers' workflow, 
a fix may be released as soon as it is merged.  A Patch should be limited expressly to the bug fix and not include anything unrelated.

## Releasing a New Version

Release branches are used to isolate a stable version of kubevirt-velero-plugin.  Git tags are used within these 
release branches to track incrementing of Minor and Patch versions.  When a Major version is incremented, a new 
stable branch should be created corresponding to the release.

- Release branches should adhere to the `release-v#.#` pattern.

- Tags should adhere to the `v#.#.#(-alpha.#)` pattern.

When creating a new release branch, follow the below process.  This assumes that `origin` references a fork of 
`kubevirt/kubevirt-velero-plugin` and you have added the main repository as the remote alias `<upstream>`.  
If you have cloned `kubevirt/kubevirt-velero-plugin` directly, omit the `<upstream>` alias.

1. Make sure you have the latest upstream code

   `$ git fetch <upstream>`

1. Create and checkout the release branch locally

   `$ git checkout -b release-v#.# upstream/main`

   e.g. `$ git checkout -b release-v1.1 upstream/main`

1. Create an annotated tag corresponding to the version

   `$ git tag -a -m "v#.#.#" v#.#.#`

   e.g. `$ git tag -a -m "v1.1.0" v1.1.0`

1. Push the new branch and tag to the main kvp repo at the same time.  (If you have cloned the main repo directly, use `origin` for <`upstream`>)

   `$ git push <upstream> release-v#.# v#.#.#`

   e.g. `$git push upstream release-v1.1 v1.1.0`

1. Generate release description. Set `PREREF` and `RELREF` shell variables to previous and current release tag, respectively.

```bash
$ export RELREF=v#.#.#
$ export PREREF=v#.#.#
$ ./hack/build/release-description.sh ${RELREF} ${PREREF}
```

CI will be triggered when a tag matching `v#.#.#` is pushed *AND* the commit changed. So you cannot simply make 
a new tag and push it, this will not trigger the CI. So for the patch release it is good to add a file, for example a
manual-release-notes, and push this commit together with a tag. The automation will handle release artifact testing, 
building, and publishing.

Following the release, `./hack/build/release-description.sh ${RELREF} ${PREREF}` should be executed to generate 
a GitHub release description template.  The `Notable Changes` section should be filled in manually, briefly listing 
major changes that the new release includes.  Copy/Paste this template into the corresponding github release.

## Promoting the release
The CI will create the release as a 'pre-release' and as such it will not show up as the latest release in Github. 
In order to promote it to a regular release go to [kubevirt-velero-plugin Github](https://github.com/kubevirt/kubevirt-velero-plugin) and click on releases on the right hand side. This will list all the releases including the new pre-release. Click edit on the pre-release (if you have permission to do so). This will open up the release editor. You can put the release description in the test area field, and uncheck the 'This is a pre-release' checkbox. Click Update release to promote to a regular release.

## Images

Ensure that the new images are available in quay.io/repository/kubevirt/container-name?tab=tags and that the version you specified in the tag is available

* [kubevirt-velero-plugin](https://quay.io/repository/kubevirt/kubevirt-velero-plugin?tab=tags)

## PROW CI

Track the CI job for the pushed tag.  Navigate to the [kubevirt-velero-plugin PROW postsubmit dashboard](https://prow.ci.kubevirt.io/?repo=kubevirt%2Fkubevirt-velero-plugin&type=postsubmit) and you can select the releases from there.