#!/usr/bin/env bash

set -eEuo pipefail

BINARY_PATH="${RUNNER_TEMP}/ratchet"

# Compute OS
OS=""
if [[ "${RUNNER_OS}" == "Linux" ]]; then
  OS="linux"
elif [[ "${RUNNER_OS}" == "macOS" ]]; then
  OS="darwin"
else
  echo "::error::Unsupported operating system ${RUNNER_OS}"
  exit 1
fi
echo "::debug::Computed OS: ${OS}"

# Compute arch
ARCH=""
if [[ "${RUNNER_ARCH}" == "X64" ]]; then
  ARCH="amd64"
elif [[ "${RUNNER_ARCH}" == "ARM64" ]]; then
  ARCH="arm64"
else
  echo "::error::Unsupported system architecture ${RUNNER_ARCH}"
  exit 1
fi
echo "::debug::Computed arch: ${ARCH}"

# Compute version
VERSION="${VERSION:-"latest"}"
if [[ "${VERSION}" == "latest" ]]; then
  VERSION=""
fi
echo "::debug::Computed version: ${VERSION}"

# Download the file
gh --repo sethvargo/ratchet release download "${VERSION}" \
  --pattern "ratchet_*_${OS}_${ARCH}.tar.gz" \
  --clobber \
  --output "-" \
  | tar -xzf - -C "${RUNNER_TEMP}"

# Mark as executable
chmod +x "${BINARY_PATH}"

# Save the result to an output.
echo "::debug::Downloaded binary to ${BINARY_PATH}"
echo "binary-path=${BINARY_PATH}" >> "${GITHUB_OUTPUT}"
