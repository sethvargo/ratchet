# Ratchet

![ratchet logo](docs/ratchet.png)

Ratchet is a tool for improving the security of CI/CD workflows by automating
the process of pinning and unpinning upstream versions. It's like Bundler,
Cargo, Go modules, NPM, Pip, or Yarn, but for CI/CD workflows. Ratchet supports:

-   Circle CI
-   GitHub Actions
-   GitLab CI
-   Google Cloud Build
-   Harness Drone
-   Tekton

**⚠️ Warning!** The README corresponds to the `main` branch of ratchet's
development, and it may contain unreleased features.


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


## Installation

There are a few options for installing ratchet:

-   Via homebrew:

    ```sh
    brew install ratchet
    ```

    Note this option is community supported and may not be the latest
    available verson.

-   As a single-static binary from the [releases page][releases].
-   As a container image from the [container registry][containers].
-   Via nix:

    ```sh
    nix run 'github:NixOS/nixpkgs/nixpkgs-unstable#ratchet' -- --help
    ```

    Note this option is community supported and may not be the latest
    available version.

-   Compiled from source yourself. Note this option is not supported.


## Usage

For more information about available commands and options, run a command with
`-help` to use detailed usage instructions. Also see [CLI Options](#cli-options).

#### Pin

The `pin` command pins to specific versions:

```shell
# pin the input file
ratchet pin workflow.yml

# pin a circleci file
ratchet pin -parser circleci circleci.yml

# pin a cloudbuild file
ratchet pin -parser cloudbuild cloudbuild.yml

# pin a drone file
ratchet pin -parser drone drone.yml

# pin a gitlab file
ratchet pin -parser gitlabci gitlabci.yml

# output to a tekton file
ratchet pin -out -parser tekton tekton.yml

# output to a different path
ratchet pin -out workflow-compiled.yml workflow.yml
```

#### Unpin

The `unpin` command unpins any pinned versions:

```shell
# unpin the input file
ratchet unpin workflow.yml

# output to a different path
ratchet unpin -out workflow.yml workflow-compiled.yml
```

#### Update

The `update` command updates all versions to the latest matching constraint:

```shell
# update the input file
ratchet update workflow.yml

# update a circleci file
ratchet update -parser circleci circleci.yml

# update a cloudbuild file
ratchet update -parser cloudbuild cloudbuild.yml

# output to a different path
ratchet update -out workflow-compiled.yml workflow.yml
```

#### Upgrade

> [!NOTE]
> This command only works with GitHub Actions references. It does not support
> container or Docker-based references.

The `upgrade` command upgrades all versions to the latest version, changing the
ratchet comment and also updating the ref.

```shell
# upgrade the input file
ratchet upgrade workflow.yml

# output to a different path
ratchet upgrade -out workflow-compiled.yml workflow.yml
```

> [!NOTE]
> Performs an `update` if the constraint ref is for a branch.

#### Check

The `check` command checks if all versions are pinned, exiting with a non-zero
error code when entries are not pinned:

```shell
ratchet check workflow.yml
```

## Examples

#### CI/CD workflow

Ratchet is distributed as a very small container, so you can use it as a step
inside CI/CD jobs. Here is a GitHub Actions example:

```yaml
jobs:
  my_job:
    runs-on: 'ubuntu-latest'
    name: 'ratchet'
    steps:
      - uses: 'actions/checkout@755da8c3cf115ac066823e79a1e1788f8940201b' # ratchet:actions/checkout@v3

      # Example of pinning:
      - uses: 'docker://ghcr.io/sethvargo/ratchet:latest'
        with:
          args: 'pin .github/workflows/my-workflow.yml'

      # Example of checking versions are pinned:
      - uses: 'docker://ghcr.io/sethvargo/ratchet:latest'
        with:
          args: 'check .github/workflows/my-workflow.yml'
```

This same pattern can be extended to other CI/CD systems that support
container-based runtimes. For non-container-based runtimes, download the `ratchet` binary from [GitHub Releases][releases].

#### Runnable container CLI

Ratchet can run directly from a container on your local system:

```shell
docker run -it --rm -v "${PWD}:${PWD}" -w "${PWD}" ghcr.io/sethvargo/ratchet:latest COMMAND
```

Create a shell alias to make this easier:

```shell
function ratchet {
  docker run -it --rm -v "${PWD}:${PWD}" -w "${PWD}" ghcr.io/sethvargo/ratchet:latest "$@"
}
```


## Auth

-   The container resolver uses default "keychain" auth, which looks for local
    system auth, similar to the Docker and gcloud CLIs.

-   The GitHub resolver defaults to public github.com. Provide an oauth access
    token with appropriate permissions via the `GITHUB_TOKEN` environment
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
    floating tag or label, such as `actions/checkout@v4` or `ubuntu:22.04`.

-   **Pinned version** - A pinned version is an absolute hashed reference, such
    as `actions/checkout@2541b1294d2704b0964813337f33b291d3f8596b` or
    `ubuntu@sha256:82becede498899ec668628e7cb0ad87b6e1c371cb8a1e597d83a47fac21d6af3`.


## Known issues

-   Indentation is always set to 2 spaces. The upstream YAML library does not
    capture pre-parsing indentation. Thus, all files will be saved with 2 spaces
    for indentation.

-   Does not support resolving values in anchors or aliases. This is technically
    possible, but most CI systems also don't support these advanced YAML
    features.

    Similarly, Ratchet does not support expansion or inteprolation, since those
    values cannot be guaranteed to be known at compile time. For example,
    Ratchet will ignore the following `${{ }}` reference in a GitHub Actions
    workflow:

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

[containers]: https://github.com/sethvargo/ratchet/pkgs/container/ratchet
[releases]: https://github.com/sethvargo/ratchet/releases
