#!/usr/bin/env bash
# Fails when the chart version in Chart.yaml already exists in the OCI
# registry, so a release never overwrites a published chart.
#
#   check-chart-unpublished.sh <chart dir> <oci repository>
#   check-chart-unpublished.sh charts/odoopack oci://ghcr.io/acme/charts
set -euo pipefail

chart_dir=$1
repository=$2

metadata=$(helm show chart "$chart_dir")
name=$(awk '/^name:/ { print $2 }' <<<"$metadata")
version=$(awk '/^version:/ { print $2 }' <<<"$metadata")

# The check runs anonymously against the public chart. Only "not found" means
# the version is free; any other error (denied, network) must not pass as such.
if output=$(helm show chart "$repository/$name" --version "$version" 2>&1); then
  echo "::error file=$chart_dir/Chart.yaml::Chart $name $version already exists in $repository. Bump version in $chart_dir/Chart.yaml before releasing."
  exit 1
fi
if ! grep -q "not found" <<<"$output"; then
  echo "$output"
  echo "::error::Could not check whether chart $name $version exists in $repository."
  exit 1
fi
echo "Chart $name $version is not published yet."
