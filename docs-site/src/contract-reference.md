# Smart Contract Reference

This reference documents the public Soroban interfaces, data structures, and error codes for both CareFund smart contracts.

## 1. Provider Registry (`contracts/provider-registry`)

### Data Structures

#### `ActorStatus`
```rust
#[contracttype]
pub enum ActorStatus {
    Active,
    Suspended,
    Revoked,
}
```

#### `ProviderRecord`
```rust
#[contracttype]
pub struct ProviderRecord {
    pub status: ActorStatus,
    pub provider_ref: BytesN<32>,
}
```

#### `AttesterRecord`
```rust
#[contracttype]
pub struct AttesterRecord {
    pub provider: Address,
    pub status: ActorStatus,
    pub credential_ref: BytesN<32>,
}
```

### Callable Functions

| Function | Parameters | Authorization | Description |
|---|---|---|---|
| `initialize` | `admin: Address` | None (one-time) | Initializes registry with admin |
| `get_admin` | None | None | Returns admin address |
| `register_provider` | `admin: Address, provider: Address, provider_ref: BytesN<32>` | Admin | Registers new provider as Active |
| `suspend_provider` | `admin: Address, provider: Address` | Admin | Sets provider status to Suspended |
| `reinstate_provider` | `admin: Address, provider: Address` | Admin | Sets provider status back to Active |
| `revoke_provider` | `admin: Address, provider: Address` | Admin | Permanently sets status to Revoked |
| `register_attester` | `admin: Address, attester: Address, provider: Address, credential_ref: BytesN<32>` | Admin | Registers attester for an active provider |
| `suspend_attester` | `admin: Address, attester: Address` | Admin | Sets attester status to Suspended |
| `reinstate_attester` | `admin: Address, attester: Address` | Admin | Sets attester status back to Active |
| `revoke_attester` | `admin: Address, attester: Address` | Admin | Permanently sets attester to Revoked |
| `is_provider_active` | `provider: Address` | Read-only | Returns boolean active status |
| `check_attester` | `attester: Address, provider: Address` | Read-only | Returns boolean authorization |

### Errors (`RegistryError`)

- `1: AlreadyInitialized`
- `2: Unauthorized`
- `3: ProviderNotFound`
- `4: ProviderAlreadyExists`
- `5: ProviderAlreadySuspended`
- `6: ProviderAlreadyActive`
- `7: ProviderAlreadyRevoked`
- `8: AttesterNotFound`
- `9: AttesterAlreadyExists`
- `10: AttesterAlreadySuspended`
- `11: AttesterAlreadyActive`
- `12: AttesterAlreadyRevoked`
- `13: ProviderNotActive`
- `14: InvalidState`

---

## 2. Care Agreement (`contracts/care-agreement`)

### Data Structures

#### `AgreementState`
```rust
#[contracttype]
pub enum AgreementState {
    Requested,
    Funded,
    CareConfirmed,
    Disputed,
    Cancelled,
    Expired,
    Refunded,
    Settled,
}
```

#### `Agreement`
```rust
#[contracttype]
pub struct Agreement {
    pub sponsor: Address,
    pub provider: Address,
    pub attester: Address,
    pub patient_ref_commitment: BytesN<32>,
    pub service_commitment: BytesN<32>,
    pub funding_amount: i128,
    pub settlement_amount: i128,
    pub funding_deadline: u64,
    pub care_deadline: u64,
    pub dispute_window_secs: u64,
    pub state: AgreementState,
    pub attestation_commitment: Option<BytesN<32>>,
    pub attested_by: Option<Address>,
    pub attested_at: Option<u64>,
    pub dispute_origin: MaybeDisputeOrigin,
    pub dispute_opened_by: Option<Address>,
    pub dispute_opened_at: Option<u64>,
    pub created_at: u64,
}
```

### Callable Functions

| Function | Parameters | Authorization | Description |
|---|---|---|---|
| `initialize` | `admin, provider_registry, settlement_asset` | None (one-time) | Initializes agreement contract |
| `create_agreement` | Eleven agreement arguments | Sponsor or Provider | Creates new care agreement in Requested state |
| `get_agreement` | `agreement_id: u64` | Read-only | Fetches complete agreement record |
| `fund` | `agreement_id: u64, sponsor: Address` | Sponsor | Transfers funding amount into contract escrow |
| `cancel` | `agreement_id: u64, provider: Address` | Provider | Cancels an un-funded agreement |
| `expire` | `agreement_id: u64` | Anyone | Expires agreement if deadlines passed |
| `attest_care` | `agreement_id: u64, attester: Address, attestation_commitment: BytesN<32>` | Attester | Submits verified care delivery hash |
| `open_dispute` | `agreement_id: u64, caller: Address` | Sponsor or Provider | Opens dispute during dispute window |
| `settle` | `agreement_id: u64, sponsor: Address` | Sponsor | Releases settlement tokens to provider |
| `resolve_dispute` | `agreement_id: u64, admin: Address, resolution: DisputeResolution` | Admin | Adjudicates dispute (Resume, Settle, Refund) |

### Errors (`AgreementError`)

- `1: AlreadyInitialized`
- `2: Unauthorized`
- `3: AgreementNotFound`
- `4: InvalidState`
- `5: InvalidAmount`
- `6: InvalidDeadline`
- `7: ProviderNotActive`
- `8: AttesterNotAuthorized`
- `9: FundingDeadlineNotReached`
- `10: FundingDeadlineNotPassed`
- `11: CareDeadlineNotPassed`
- `12: DisputeWindowActive`
- `13: DisputeWindowClosed`
- `14: InvalidResolution`
- `15: TransferFailed`
- `16: ArithmeticOverflow`
