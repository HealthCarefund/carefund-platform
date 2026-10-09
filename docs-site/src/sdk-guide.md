# SDK Guide

The CareFund TypeScript packages provide type definitions, contract bindings, transaction assembly, and wallet connection management.

## Package Architecture

CareFund divides TypeScript tooling into two packages:
- **`@carefund/types`**: Zero-dependency domain primitives, branded types, and schema validations.
- **`@carefund/sdk`**: Client configuration validation, Freighter wallet adapter, transaction simulation, submission, and generated Soroban contract clients.

---

## 1. Domain Types (`@carefund/types`)

CareFund uses TypeScript branded types to eliminate primitive obsession and prevent invalid address or hash arguments:

```typescript
import { StellarAddress, CommitmentHash, AgreementId } from "@carefund/types";

// Branded primitives enforce compile-time safety
const sponsor: StellarAddress = "GDEI32FXSM6XIOKHUXT43MOB7GTKRRZVBNVVCW6XE6DXXKL7J3LKCI3S" as StellarAddress;
const patientCommitment: CommitmentHash = "a1b2c3d4..." as CommitmentHash;
const agreementId: AgreementId = 1n as AgreementId;
```

---

## 2. Configuration Validation (`@carefund/sdk`)

All network communication starts with strict configuration verification:

```typescript
import { validateStellarClientConfig } from "@carefund/sdk";

const config = validateStellarClientConfig({
  network: "TESTNET",
  rpcUrl: "https://soroban-testnet.stellar.org",
  networkPassphrase: "Test SDF Network ; September 2015",
  providerRegistryContractId: "CCGF5Y7...",
  careAgreementContractId: "CD6NC44...",
  settlementAssetContractId: "CDLZFC3...",
});
```

If any parameter fails format checks or if `network` does not align with `networkPassphrase`, `validateStellarClientConfig` throws a descriptive error immediately.

---

## 3. Freighter Wallet Adapter

The wallet module provides a typed wrapper around the Freighter browser extension API:

```typescript
import { FreighterWalletAdapter, WalletError } from "@carefund/sdk";

const wallet = new FreighterWalletAdapter();

// Check installation
const isInstalled = await wallet.isInstalled();
if (!isInstalled) {
  throw new Error("Freighter wallet is not installed");
}

// Request account connection
const publicKey = await wallet.connect();

// Sign transaction XDR
try {
  const signedXdr = await wallet.signTransaction(unsignedXdr, {
    networkPassphrase: config.networkPassphrase,
  });
} catch (err) {
  if (err instanceof WalletSigningRejectedError) {
    console.warn("User declined signature");
  } else if (err instanceof WrongNetworkError) {
    console.error("Wallet configured to incorrect network");
  }
}
```

---

## 4. Contract Bindings (`@carefund/sdk/generated`)

TypeScript contract bindings are generated from contract WASM files using `stellar contract bindings typescript`.

Bindings provide typed methods for invoking contract endpoints, handling serialization and deserialization of Soroban `ScVal` structures transparently:

```typescript
import { Client as CareAgreementClient } from "@carefund/sdk/generated/care-agreement";

const agreementClient = new CareAgreementClient({
  contractId: config.careAgreementContractId,
  rpcUrl: config.rpcUrl,
  networkPassphrase: config.networkPassphrase,
});

// Query agreement details
const agreement = await agreementClient.getAgreement({ agreement_id: 1n });
console.log("State:", agreement.state);
```
