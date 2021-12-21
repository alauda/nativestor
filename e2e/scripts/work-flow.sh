#!/usr/bin/env bash

# Copyright 2021 The Rook Authors. All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -xeEo pipefail

#############
# FUNCTIONS #
#############

function print_k8s_cluster_status() {
  kubectl cluster-info
  kubectl get pods -n kube-system
}

FUNCTION="$1"
shift # remove function arg now that we've recorded it
# call the function with the remainder of the user-provided args
# -e, -E, and -o=pipefail will ensure this scripts returns a failure if a part of the function fails
$FUNCTION "$@"
