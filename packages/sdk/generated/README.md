# Generated contract bindings

Everything under `provider-registry/` and `care-agreement/` in this directory
(except each package's `package.json`) is produced verbatim by the Stellar
CLI and must never be hand-edited. If a binding looks wrong or incomplete,
fix the Rust contract or regenerate — do not patch the generated
`src/index.ts` directly.

Handwritten SDK code that builds on these bindings (typed config, RPC
access, transaction lifecycle, and eventually the high-level CareFund
wrapper methods) lives in `packages/sdk/src`, not here.

## Regenerating

```
./scripts/generate-contract-bindings.sh
```

This was generated with:

- Stellar CLI 27.0.0
- Rust 1.98.1 (pinned by `rust-toolchain.toml`)
- `stellar contract build`, then `stellar contract bindings typescript --wasm <path> --output-dir <dir> --overwrite`
  for each of `target/wasm32v1-none/release/provider_registry.wasm` and
  `target/wasm32v1-none/release/care_agreement.wasm`.

The CLI's default output pins `@stellar/stellar-sdk` and `typescript` to
whatever it currently bundles; after regenerating, re-apply this repo's
exact pins in each package's `package.json`:

- `@stellar/stellar-sdk`: `17.1.0`
- `typescript`: `6.0.3`

(`buffer` is left at the CLI's pinned `6.0.3`, which is unrelated to the
Stellar SDK version.)

The CLI's generated `tsconfig.json` also needs `"rootDir": "./src"` added
under `compilerOptions` — TypeScript 6.0 (unlike the 5.6 the CLI was built
against) errors without it (`TS5011`) because `outDir` is set but `rootDir`
is not inferable. Re-add this line after regenerating.
