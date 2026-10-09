# Scope and Participant Roles

CareFund coordinates interactions across four distinct system roles. Every action on-chain requires explicit cryptographic authorization from the appropriate role identity.

## Participant Roles

### 1. Contract Administrator (`Admin`)
The administrator represents the governing entity or protocol deployer.
- **Responsibilities**:
  - Initializes `provider-registry` and `care-agreement` contracts.
  - Registers accredited healthcare providers in the registry.
  - Updates provider statuses (Active, Suspended, Revoked).
  - Registers authorized attesters linked to specific providers.
  - Adjudicates and resolves disputed agreements (`resolve_dispute`).
- **Authorization**: Protected on-chain by `admin.require_auth()`.

### 2. Healthcare Sponsor (`Sponsor`)
Sponsors are philanthropic organizations, donor funds, or individuals allocating capital for medical procedures.
- **Responsibilities**:
  - Creates or reviews care agreements with designated providers.
  - Deposits settlement funds into contract escrow before the funding deadline (`fund`).
  - Receives automated refund of any surplus escrow upon settlement.
  - Opens disputes if service irregularities occur (`open_dispute`).
  - Reclaims funds if agreements expire without funding or care attestation (`expire`).
- **Authorization**: Protected on-chain by `sponsor.require_auth()`.

### 3. Healthcare Provider (`Provider`)
Providers are clinics, hospitals, or accredited medical practitioners delivering care.
- **Responsibilities**:
  - Registers their Stellar address and operational reference commitment via administrator onboarding.
  - Proposes agreements specifying funding amount, settlement amount, and deadlines.
  - Cancels requested agreements if terms change prior to sponsor funding (`cancel`).
  - Receives automated token transfer of the settlement amount upon successful settlement (`settle`), and can trigger settlement once the dispute window elapses.
  - Opens disputes if attestation is improperly withheld (`open_dispute`).
- **Authorization**: Protected on-chain by `provider.require_auth()`. Must maintain `ActorStatus::Active` in `provider-registry`.

### 4. Independent Attester (`Attester`)
Attesters are neutral verification agents, such as clinical auditors, program officers, or designated verification staff.
- **Responsibilities**:
  - Reviews physical or off-chain evidence that clinical services were completed.
  - Submits on-chain attestation commitments (`attest_care`).
  - Cannot withdraw, redirect, or seize escrowed funds.
- **Authorization**: Protected on-chain by `attester.require_auth()`. Must be explicitly authorized for the target provider via `check_attester` in `provider-registry`.

---

## Operational Scope Matrix

| Action | Initiating Role | On-Chain Function | State Precondition |
|---|---|---|---|
| Initialize Registry | Admin | `initialize` | Uninitialized |
| Register Provider | Admin | `register_provider` | Admin authorized |
| Register Attester | Admin | `register_attester` | Provider active |
| Create Agreement | Sponsor or Provider | `create_agreement` | Provider active, Attester authorized |
| Deposit Escrow | Sponsor | `fund` | Requested, before `funding_deadline` |
| Cancel Agreement | Provider | `cancel` | Requested |
| Attest Care | Attester | `attest_care` | Funded, before `care_deadline` |
| Settle Agreement | Anyone (Permissionless) | `settle` | CareConfirmed, past `care_deadline + dispute_window_secs` |
| Open Dispute | Sponsor or Provider | `open_dispute` | Funded or CareConfirmed, within dispute window |
| Resolve Dispute | Admin | `resolve_dispute` | Disputed |
| Expire Agreement | Anyone (Permissionless) | `expire` | Requested (past funding deadline) or Funded (past care deadline plus dispute window without attestation) |

---

## Non-Clinical Scope Boundary

To guarantee compliance and safety:
1. **No Medical Practice**: CareFund software never determines medical necessity, diagnosis, or prescription appropriateness.
2. **No Data Custody**: CareFund never stores electronic health records, diagnostic scans, or patient contact information.
3. **Commitment Model**: All references to individuals or medical treatments are opaque 32-byte cryptographic digests computed by clinics within their own internal systems.
