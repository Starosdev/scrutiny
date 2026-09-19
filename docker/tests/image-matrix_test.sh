#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
matrix_file="${repo_root}/docker/image-matrix.json"

jq -e '
    type == "array" and length > 0
    and all(.[];
        (.image | type == "string")
        and (.dockerfile | type == "string")
        and (.platforms | contains("linux/amd64"))
        and (.platforms | contains("linux/arm64"))
        and (.qemu | type == "string")
        and (.cache_scope | type == "string")
        and (.pr_build | type == "boolean")
        and (.frontend | type == "boolean")
        and (.default_image | type == "boolean")
    )
    and ([.[].image] | length == (unique | length))
    and ([.[] | select(.pr_build == true)] | length > 0)
    and ([.[] | select(.apparmor_kind != null)] | length > 0)
    and all(.[]; (.pr_build == true or .apparmor_kind != null))
    and all(.[]; (.pr_build != true or .apparmor_kind == null))
    and ([.[] | select(.default_image == true)] | length == 1)
' "${matrix_file}" >/dev/null

manifest_files=()
while IFS= read -r path; do
    manifest_files+=( "${path}" )
done < <(jq -r '.[].dockerfile' "${matrix_file}" | sort)

repository_files=()
while IFS= read -r path; do
    repository_files+=( "${path}" )
done < <(find "${repo_root}/docker" -maxdepth 1 -type f -name 'Dockerfile*' -print | sed "s#^${repo_root}/##" | sort)

diff -u <(printf '%s\n' "${repository_files[@]}") <(printf '%s\n' "${manifest_files[@]}")

echo "Docker image matrix tests passed"
