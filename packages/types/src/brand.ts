/**
 * Nominal typing helper. String/numeric primitives that carry different
 * on-chain meanings (an agreement id vs. a raw u64 vs. a Stellar address)
 * are otherwise structurally identical and freely interchangeable in
 * TypeScript, which is exactly what we don't want at the domain boundary.
 */
export type Brand<T, B extends string> = T & { readonly __brand: B };
