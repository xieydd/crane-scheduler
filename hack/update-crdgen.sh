#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# For all commands, the working directory is the parent directory(repo root).
REPO_ROOT=$(git rev-parse --show-toplevel)
cd "${REPO_ROOT}"

CONTROLLER_GEN_PKG="sigs.k8s.io/controller-tools/cmd/controller-gen"
CONTROLLER_GEN_VER="v0.7.0"

source hack/util.sh

echo "Generating with controller-gen"
util::install_tools ${CONTROLLER_GEN_PKG} ${CONTROLLER_GEN_VER} >/dev/null 2>&1

controller-gen object:headerFile="hack/boilerplate.go.txt" rbac:roleName=manager-role crd webhook paths="$REPO_ROOT/pkg/apis/scheduling/..." output:crd:artifacts:config=deploy/manifests;

#controller-gen crd paths=./pkg/apis/scheduling/... output:crd:dir=./artifacts/deploy
#controller-gen webhook paths=./pkg/apis/scheduling/... output:webhook:dir=./artifacts/deploy

