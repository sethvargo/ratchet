# Ratchet

Ratchet is a tool for improving the security of CI/CD workflows by automating
the process of pinning and unpinning upstream versions. It's like Bundler, Cargo, Go modules, NPM, Pip, or Yarn, but for CI/CD workflows. Ratchet supports:

-   GitHub Actions
-   Google Cloud Build

## Problem statement

Most CI/CD systems are one layer of indirection away from `curl | sudo bash`.
Unless you are specifically pinning CI workflows, containers, and base images to
checksummed versions, _everything_ is mutable: GitHub labels are mutable and
Docker tags are mutable. This poses a substantial security and reliability risk.

What you're probably doing:

```yaml
uses: 'actions/checkout@v3'
# or
image: 'ubuntu:20.04'
```

What you should really be doing:

```yaml
uses: 'actions/checkout@2541b1294d2704b0964813337f33b291d3f8596b'
# or
image: 'ubuntu@sha256:47f14534bda344d9fe6ffd6effb95eefe579f4be0d508b7445cf77f61a0e5724'
```

But resolving those checksums and managing the update lifecycle is extremely
toilsome. That's what ratchet aims to solve!


## Usage

Pinning:

```shell
# pin the input file
./ratchet pin workflow.yml

# pin a cloudbuild file
./ratchet pin -parser cloudbuild cloudbuild.yml

# output to a different path
./ratchet pin -out workflow-compiled.yml workflow.yml
```

Unpinning:

```shell
# unpin the input file
./ratchet unpin workflow.yml

# output to a different path
./ratchet unpin -out workflow.yml workflow-compiled.yml
```

Updating:

```shell
# update the input file
./ratchet update workflow.yml

# update a cloudbuild file
./ratchet update -parser cloudbuild cloudbuild.yml

# output to a different path
./ratchet pin -out workflow-compiled.yml workflow.yml
```


## Auth

-   Docker uses default "keychain" auth, which looks for local system auth.
-   GitHub accepts CLI flags


## Terminology

-   **Unpinned version** - An unpinned version is a non-absolute reference to a
    floating tag or label, such as `actions/checkout@v3` or `ubuntu:20.04`.

-   **Pinned version** - A pinned version is an absolute hashed reference, such
    as `actions/checkout@2541b1294d2704b0964813337f33b291d3f8596b` or
    `ubuntu@sha256:82becede498899ec668628e7cb0ad87b6e1c371cb8a1e597d83a47fac21d6af3`.


## Known issues

-   Indentation is always set to 2 spaces. The upstream YAML library does not
    capture pre-parsing indentation. Thus, all files will be saved with 2 spaces
    for indentation.

-   Leading and trailing whitespace between nodes is not preserved. Similar
    indentation, the upstream YAML library does not capture truly empty nodes.
    Thus, blank lines may be removed between nodes. This will not affect
    multi-line values.

-   Does not support resolving values in anchors or aliases. This is technically
    possible, but most CI systems also don't support these advanced YAML
    features.


curl -sv \
  -H "Authorization: Bearer $TOKEN" \
  -H "Accept: application/vnd.docker.distribution.manifest.v2+json" \
  -H "Accept: application/vnd.docker.distribution.manifest.list.v2+json" \
  https://registry.docker.com/v2/library/ubuntu/manifests/20.04
