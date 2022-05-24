# Ratchet

![ratchet logo](docs/ratchet.png)

Ratchet is a tool for improving the security of CI/CD workflows by automating
the process of pinning and unpinning upstream versions. It's like Bundler,
Cargo, Go modules, NPM, Pip, or Yarn, but for CI/CD workflows. Ratchet supports:

-   Circle CI
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
toilsome. That's what ratchet aims to solve! Ratchet resolves and updates
unpinned references to the latest version that matches their constraint, and
then keeps a record of the original constraint.

```yaml
uses: 'actions/checkout@2541b1294d2704b0964813337f33b291d3f8596b' # ratchet:actions/checkout@v3
# or
image: 'ubuntu@sha256:47f14534bda344d9fe6ffd6effb95eefe579f4be0d508b7445cf77f61a0e5724' # ratchet:ubuntu:20.04
```


## Usage

**⚠️ Warning!** The README corresponds to the `main` branch of ratchet's
development, and it may contain unreleased features.

**Pin to specific versions:**

```shell
# pin the input file
./ratchet pin workflow.yml

# pin a circleci file
./ratchet pin -parser circleci circleci.yml

# pin a cloudbuild file
./ratchet pin -parser cloudbuild cloudbuild.yml

# output to a different path
./ratchet pin -out workflow-compiled.yml workflow.yml
```

**Unpin existing pinned versions:**

```shell
# unpin the input file
./ratchet unpin workflow.yml

# output to a different path
./ratchet unpin -out workflow.yml workflow-compiled.yml
```

**Update all versions to the latest matching constraint:**

```shell
# update the input file
./ratchet update workflow.yml

# update a circleci file
./ratchet update -parser circleci circleci.yml

# update a cloudbuild file
./ratchet update -parser cloudbuild cloudbuild.yml

# output to a different path
./ratchet pin -out workflow-compiled.yml workflow.yml
```

For more information, run a command with `-help` to use detailed usage
instructions.

**Check if all versions are pinned:**

```shell
./ratchet check workflow.yml
```

**In a CI/CD workflow:**

Ratchet is distributed as a very small container, so you can use it as a step
inside CI/CD jobs. Here is a GitHub Actions example:

```yaml
jobs:
  my_job:
    runs-on: 'ubuntu-latest'
    name: 'ratchet'
    steps:
      - uses: 'actions/checkout@2541b1294d2704b0964813337f33b291d3f8596b' # ratchet:actions/checkout@v3

      # Example of pinning.
      - uses: 'docker://ghcr.io/sethvargo/ratchet:0.2.1'
        with:
          args: ['pin', '.github/workflows/my-workflow.yml']

      # Example of checking versions are pinned.
      - uses: 'docker://ghcr.io/sethvargo/ratchet:0.2.1'
        with:
          args: ['check', '.github/workflows/my-workflow.yml']
```

This same pattern can be extended to other CI/CD systems that support
container-based runtimes.


## Installation

There are a few options for installing ratchet:

-   As a single-static binary from the [releases page](https://github.com/sethvargo/ratchet/releases).
-   As a container image from the [container registry](https://github.com/sethvargo/ratchet/pkgs/container/ratchet).
-   Compiled from source yourself. Note this option is not supported.


## Auth

-   The container resolver uses default "keychain" auth, which looks for local
    system auth, similar to the Docker and gcloud CLIs.

-   The GitHub resolver defaults to public github.com. Provide an oauth access
    token with appropriate permissions via the `ACTIONS_TOKEN` environment
    variable. To use a GitHub Enterprise installation, set the
    `ACTIONS_BASE_URL` and `ACTIONS_UPLOAD_URL` environment variables to point
    your instance.


## Excluding

There may be instances in which you want to exclude a particular reference from
being pinned. You can use the `ratchet:exclude` annotation as a line comment and
ratchet will not process that reference:

```yaml
uses: 'actions/checkout@v3' # ratchet:exclude
```

There **cannot** be any spaces in the exclusion string, and the exclusion string
only applies to the line on which it appears.


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

    Similarly, Ratchet does not support matrix-style expansion, since those
    values cannot be guaranteed to be known at compile time. For example,
    Ratchet will error on the following GitHub Actions workflow:

    ```yaml
    jobs:
      my_job:
        strategy:
          matrix:
            version:
            - '1'
            - '2'

        steps:
          - uses: 'actions/checkout@v${{ matrix.version }}'
    ```
