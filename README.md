# Hermit CI

Hermit is an experimental CI server for building container images from GitHub repositories. Hermit's schtick is to run builds in ephemeral virtual machines that are limited to HTTP/HTTPS network connections through a custom proxy.

The proxy has several features:

* Limit the URLs that can be accessed during the build. [Example rules](https://github.com/thepwagner-org/hermit-vm-kernel/blob/bcbaa2dbd29207199ef7553eab1d9c41c9905bea/.hermit/rules.yaml).
* Fetch assets from a shared cache (Redis) to reduce network traffic.
* Record every request made during the build. [Example snapshot](https://github.com/thepwagner-org/hermit-vm-kernel/tree/437e8b8c0d289176f1c646ca39f12afaeab7c829/.hermit/network).
* Replay a recording and restrict other network access, to reproduce builds in a hermetic environment.
* Generate a CA keypair at launch, for inspecting HTTPS traffic.

The idea was to satisfy the [Hermetic requirement from SLSA level 4](https://slsa.dev/requirements#build-requirements), without requiring purpose-built build tools (e.g. bazel). Hermit runs existing build tools, and locks them to a fixed snapshot of the internet.

Builder dependencies:
* https://github.com/thepwagner-org/hermit-vm-kernel
* https://github.com/thepwagner-org/hermit-vm-root

Hermit was designed to run on my home infra, and is not intended to be used.
