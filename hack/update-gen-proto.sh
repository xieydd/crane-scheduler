#!/usr/bin/env bash


# This script generates `*/api.pb.go` from the protobuf file `*/api.proto`.
# Example:
#   kube::protoc::generate_proto "${PREDICTION_V1}"

set -o errexit
set -o nounset
set -o pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../" && pwd -P)"

source "${ROOT}/hack/protoc.sh"
