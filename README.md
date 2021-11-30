# Hermit CI

Hermit is an experimental CI server for building container images from GitHub repositories. Hermit's schtick is to run builds in ephemeral virtual machines that are limited to HTTP/HTTPS network connections through a custom proxy.
The idea was to satisfy the [Hermetic requirement from SLSA level 4](https://slsa.dev/requirements#build-requirements), without requiring purpose-built build tools (e.g. bazel). Hermit runs existing build tools, and locks them to a fixed snapshot of the internet.

Hermit was designed to run on my home infra, and is not intended to be used.

Hermit's proxy has several features:

* Limit the URLs that can be accessed during the build. [Example rules](https://github.com/thepwagner-org/hermit-vm-kernel/blob/bcbaa2dbd29207199ef7553eab1d9c41c9905bea/.hermit/rules.yaml).
* Fetch assets from a shared cache (Redis) to reduce network traffic.
* Record every request made during the build. [Example snapshot](https://github.com/thepwagner-org/hermit-vm-kernel/tree/437e8b8c0d289176f1c646ca39f12afaeab7c829/.hermit/network).
* Restrict network access to replaying a recording, to reproduce builds in a hermetic environment.
* Generate a CA keypair at launch, for intercepting HTTPS traffic.

## Flow

Hermit is triggered by GitHub push events.

### Container builds

1. If the push was made by Hermit, or was made to the default branch, Hermit will run the build with the proxy limited to requests in the current snapshot. This is a hermetic build.
1. If the push was not made by Hermit, Hermit will run the build with the proxy following the specified rules. If Hermit detects network changes, it will push a commit to amend the snapshot.
1. The built container is scanned using [aquasecurity/trivy](https://github.com/aquasecurity/trivy). This is hermetic. [Sample result](https://github.com/thepwagner-org/hermit-vm-root/pull/17#issuecomment-981090786).
1. If the push was made to the default branch, the built container is pushed to the registry.

### GitOps

I keep a `gitops` repo full of [kustomization files](https://kubernetes.io/docs/tasks/manage-kubernetes-objects/kustomization/) for hosted services.
Any containers with active deployments, built by Hermit or externally, will be raised by Renovate as PRs against this repository. It has a simplified flow:

1. On push, find all images affected by the current branch. Scan every image and post the result.


### Builder/guest dependencies:

* https://github.com/thepwagner-org/hermit-vm-kernel
* https://github.com/thepwagner-org/hermit-vm-root
