# Provider and Attester Registry

The `provider-registry` contract (`contracts/provider-registry`) serves as the foundational trust root for healthcare participants in CareFund. It maintains verifiable credentials and accreditation statuses for clinics and independent attesters.

## Core Concepts

### Actor Statuses
Both healthcare providers and attesters exist in one of three explicit lifecycle states:

```rust
pub enum ActorStatus {
    Active,
    Suspended,
    Revoked,
}
```

- **Active**: Participant is fully authorized to propose agreements or submit care attestations.
- **Suspended**: Participant is temporarily restricted (for administrative review or credential renewal). Existing active agreements continue, but new agreements cannot be created.
- **Revoked**: Participant is permanently barred. Revocation is terminal and cannot be reinstated.

### Records Structure

#### Provider Record
A provider represents an accredited healthcare delivery organization:
```rust
pub struct ProviderRecord {
    pub status: ActorStatus,
    pub provider_ref: BytesN<32>, // Opaque hash of clinic registration
}
```

#### Attester Record
An attester represents an authorized verification officer bound directly to an authorized provider:
```rust
pub struct AttesterRecord {
    pub provider: Address,        // Associated provider organization
    pub status: ActorStatus,
    pub credential_ref: BytesN<32>, // Opaque hash of clinical license/credential
}
```

---

## Contract Interface

The trait `ProviderRegistryTrait` specifies all callable endpoints:

### Initialization and Administration
- `initialize(admin: Address)`: Assigns the contract administrator. Callable only once.
- `get_admin() -> Result<Address, RegistryError>`: Returns the current admin address.

### Provider Lifecycle Management
- `register_provider(admin: Address, provider: Address, provider_ref: BytesN<32>)`: Onboards a new clinic with status `Active`. Requires admin authorization.
- `suspend_provider(admin: Address, provider: Address)`: Transitions status from `Active` to `Suspended`.
- `reinstate_provider(admin: Address, provider: Address)`: Restores a `Suspended` provider back to `Active`.
- `revoke_provider(admin: Address, provider: Address)`: Permanently sets status to `Revoked`.

### Attester Lifecycle Management
- `register_attester(admin: Address, attester: Address, provider: Address, credential_ref: BytesN<32>)`: Links an attester to a specified provider. Requires admin authorization. The target provider must be currently `Active`.
- `suspend_attester(admin: Address, attester: Address)`: Temporarily suspends attestation privileges.
- `reinstate_attester(admin: Address, attester: Address)`: Restores attester privileges if the parent provider is `Active`.
- `revoke_attester(admin: Address, attester: Address)`: Permanently revokes attester privileges.

### Verification Queries
- `is_provider_active(provider: Address) -> bool`: Read-only helper returning `true` if the provider exists and is `Active`.
- `check_attester(attester: Address, provider: Address) -> bool`: Read-only helper verifying that the attester exists, is `Active`, and is explicitly bound to the provided `provider` address.
- `get_provider(provider: Address) -> Option<ProviderRecord>`: Fetches full provider record.
- `get_attester(attester: Address) -> Option<AttesterRecord>`: Fetches full attester record.

---

## Cross-Contract Interaction

When `care-agreement` executes `create_agreement` or `fund`, it invokes `is_provider_active` and `check_attester` as real cross-contract calls:
- If a provider is suspended between agreement proposal and funding, the agreement cannot be funded.
- If an attester is revoked or not bound to the provider, agreement creation fails immediately with `AttesterNotAuthorized`.

---

## Storage and State TTL Management

Soroban requires state archival management via Time-To-Live (TTL) extensions:
- **Instance TTL**: Threshold `120,960` ledgers (~7 days). Extended to `518,400` ledgers (~30 days).
- **Persistent Entries**: Every provider and attester entry checks TTL thresholds on write and extends lifetime accordingly.

---

## Registry Errors

| Code | Variant | Cause |
|---|---|---|
| 1 | `AlreadyInitialized` | Contract was previously initialized |
| 2 | `Unauthorized` | Caller is not the authorized administrator |
| 3 | `ProviderNotFound` | Target provider address does not exist |
| 4 | `ProviderAlreadyExists` | Target provider address is already registered |
| 5 | `ProviderAlreadySuspended` | Cannot suspend an already suspended provider |
| 6 | `ProviderAlreadyActive` | Cannot reinstate an active provider |
| 7 | `ProviderAlreadyRevoked` | Cannot modify a revoked provider |
| 8 | `AttesterNotFound` | Target attester address does not exist |
| 9 | `AttesterAlreadyExists` | Target attester address is already registered |
| 10 | `AttesterAlreadySuspended` | Cannot suspend an already suspended attester |
| 11 | `AttesterAlreadyActive` | Cannot reinstate an active attester |
| 12 | `AttesterAlreadyRevoked` | Cannot modify a revoked attester |
| 13 | `ProviderNotActive` | Parent provider is suspended or revoked |
| 14 | `InvalidState` | Operation violates state preconditions |
