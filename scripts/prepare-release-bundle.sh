#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 3 ]]; then
  echo "usage: $0 <stage-dir> <binary> <node-runtime-root>" >&2
  exit 2
fi

stage_dir=$1
binary=$2
node_root=$3

if [[ ! -f "$binary" ]]; then
  echo "release binary is missing: $binary" >&2
  exit 1
fi
if [[ ! -f UI/GUI/dist/index.html || ! -f UI/TUI/dist/tui.mjs ]]; then
  echo "build UI/GUI and UI/TUI artifacts before packaging" >&2
  exit 1
fi
if [[ ! -x "$node_root/bin/node" ]]; then
  echo "Node runtime is missing bin/node: $node_root" >&2
  exit 1
fi
case "$node_root" in
  /|/usr|/usr/local)
    echo "refusing to package a system-wide Node root: $node_root" >&2
    exit 1
    ;;
esac

mkdir -p "$stage_dir/UI/GUI" "$stage_dir/UI/TUI" "$stage_dir/runtime"
install -m 0755 "$binary" "$stage_dir/lh"
cp -R UI/GUI/dist "$stage_dir/UI/GUI/dist"
cp -R UI/TUI/dist "$stage_dir/UI/TUI/dist"
cp -R "$node_root" "$stage_dir/runtime/node"
install -m 0755 packaging/unix/install.sh "$stage_dir/install.sh"
