#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -ne 2 ]; then
  echo "Usage: $0 <image1> <image2>"
  echo "Example: $0 python:3.10-alpine python:3.10-slim"
  exit 1
fi

IMAGE1="$1"
IMAGE2="$2"

TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

human_size() {
  numfmt --to=si --suffix=B "$1"
}

export_rootfs() {
  local image="$1"
  local output="$2"
  local cid=""

  echo "Pulling $image..."
  docker pull "$image" >/dev/null

  echo "Creating container from $image..."
  cid="$(docker create "$image")"

  cleanup_container() {
    docker rm "$cid" >/dev/null 2>&1 || true
  }

  trap cleanup_container RETURN

  echo "Exporting root filesystem for $image..."
  docker export "$cid" -o "$output"

  docker rm "$cid" >/dev/null
  trap - RETURN
}

TAR1="$TMPDIR/image1-rootfs.tar"
TAR2="$TMPDIR/image2-rootfs.tar"

export_rootfs "$IMAGE1" "$TAR1"
export_rootfs "$IMAGE2" "$TAR2"

SIZE1="$(stat -c%s "$TAR1")"
SIZE2="$(stat -c%s "$TAR2")"

DIFF=$(( SIZE1 - SIZE2 ))
ABS_DIFF="${DIFF#-}"

echo
echo "Root filesystem tar sizes:"
echo "  $IMAGE1: $(human_size "$SIZE1") ($SIZE1 bytes)"
echo "  $IMAGE2: $(human_size "$SIZE2") ($SIZE2 bytes)"

echo
if [ "$SIZE1" -gt "$SIZE2" ]; then
  echo "$IMAGE1 is larger by $(human_size "$ABS_DIFF") ($ABS_DIFF bytes)"
elif [ "$SIZE2" -gt "$SIZE1" ]; then
  echo "$IMAGE2 is larger by $(human_size "$ABS_DIFF") ($ABS_DIFF bytes)"
else
  echo "Both exported root filesystems have the same size."
fi