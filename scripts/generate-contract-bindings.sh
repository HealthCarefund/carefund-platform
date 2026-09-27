#!/usr/bin/env bash
# Regenerates the TypeScript contract bindings in packages/sdk/generated/
# from the actual compiled contract WASM. Run this after any change to
# contracts/provider-registry or contracts/care-agreement.
#
# Requires: Rust 1.98.1 (pinned by rust-toolchain.toml), Stellar CLI 27.0.0.
#
# The generated output (packages/sdk/generated/**) is produced entirely by
# `stellar contract bindings typescript` and must never be hand-edited;
# only its package.json dependency versions are adjusted afterward to match
# this repo's pinned toolchain (see that directory's README).
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."

echo "Stellar CLI: $(stellar --version | head -n1)"
echo "Rust:        $(rustc --version)"

stellar contract build

stellar contract bindings typescript \
  --wasm target/wasm32v1-none/release/provider_registry.wasm \
  --output-dir packages/sdk/generated/provider-registry \
  --overwrite

stellar contract bindings typescript \
  --wasm target/wasm32v1-none/release/care_agreement.wasm \
  --output-dir packages/sdk/generated/care-agreement \
  --overwrite

echo
echo "Bindings regenerated. Re-apply the package.json dependency pins"
echo "(@stellar/stellar-sdk 17.1.0, typescript 6.0.3) if this overwrote them,"
echo "then run: pnpm install"
