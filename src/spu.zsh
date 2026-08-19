#!/usr/bin/env zsh

# spu — the single dispatcher for the soppy-utils collection.
#
# Routes `spu <util> [...args]` to the matching util's entry function, or
# prints the overview usage when given no util (or an explicit help request).

__spu_dir="${(%):-%N}"
__spu_dir="${__spu_dir:A:h}"

source "$__spu_dir/usage/spu.zsh"
source "$__spu_dir/vscode.sh"
source "$__spu_dir/license.sh"
source "$__spu_dir/utils/style.zsh"

spu() {
  local util="${1:-}"

  case "$util" in
    ""|help|-h|--help)
      # `-h` requests brief help; `--help` requests the detailed overview.
      [[ "$util" == "--help" ]] && usage_spu true || usage_spu false
      ;;
    vscode)
      shift
      vscode_main "$@"
      ;;
    license)
      shift
      license_main "$@"
      ;;
    *)
      err "spu: unknown util: $util"
      usage_spu false >&2
      return 1
      ;;
  esac
}
