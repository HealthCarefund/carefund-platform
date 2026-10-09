# CareAgreement Lifecycle

The `care-agreement` contract (`contracts/care-agreement`) orchestrates the conditional escrow and multi-party workflow for individual medical procedures.

## State Transition Model

```
               +---------------+
               |   Requested   |
               +-------+-------+
                       |
        +--------------+--------------+
        | (fund)                      | (cancel or expire)
        v                             v
  +-----------+                 +-----------+
  |  Funded   |                 | Cancelled | / Expired
  +-----+-----+                 +-----------+
        |
        | (attest_care)
        v
+---------------+
| CareConfirmed | <---+
+-------+-------+     |
        |             |
        | (settle)    | (resolve: Resume)
        v             |
  +-----------+       |
  |  Settled  |       |
  +-----------+       |
                      |
    (open_dispute)    |
         +------------+------------+
         |                         |
         v                         v
   +-----------+             +-----------+
   | Disputed  | --(Refund)->| Refunded  |
   +-----------+             +-----------+
```

---

## Lifecycle States

| State | Description | Escrow Balance |
|---|---|---|
| `Requested` | Agreement terms defined, awaiting sponsor deposit | 0 |
| `Funded` | Sponsor tokens escrowed in contract; clinic delivers care | Equal to `funding_amount` |
| `CareConfirmed` | Attester recorded verified care delivery hash | Equal to `funding_amount` |
| `Settled` | Escrow paid out to provider; terminal state | 0 (or retained difference) |
| `Cancelled` | Provider cancelled un-funded proposal; terminal state | 0 |
| `Expired` | Deadline lapsed without required action; terminal state | 0 (refunded to sponsor if funded) |
| `Disputed` | Formal disagreement raised; actions frozen pending admin | Locked in contract |
| `Refunded` | Admin dispute adjudication returned funds to sponsor | 0 |

---

## Detailed Step-by-Step Flow

### 1. Agreement Creation (`create_agreement`)
A sponsor or provider defines the contract parameters:
- `sponsor`: Funder Stellar account.
- `provider`: Healthcare provider Stellar account.
- `attester`: Designated neutral attester account.
- `patient_ref_commitment`: 32-byte hash identifying the patient.
- `service_commitment`: 32-byte hash identifying planned procedure.
- `funding_amount`: Total token amount required from sponsor.
- `settlement_amount`: Amount to be disbursed to provider on completion.
- `funding_deadline`: Unix timestamp by which deposit must occur.
- `care_deadline`: Unix timestamp by which care must be delivered and attested.
- `dispute_window_secs`: Grace window (in seconds) for objections.

**Validations Enforced**:
- Provider must be `Active` in `provider-registry`.
- Attester must be authorized for this provider.
- `funding_amount >= settlement_amount > 0`.
- `funding_deadline < care_deadline`.
- `care_deadline + dispute_window_secs` must not cause arithmetic overflow.

### 2. Funding (`fund`)
- **Caller**: Sponsor (`sponsor.require_auth()`).
- **Prerequisites**: Agreement must be in `Requested` state and current ledger timestamp must be `<= funding_deadline`.
- **Execution**: Invokes Stellar Asset Contract (`SAC.transfer`) to transfer `funding_amount` from sponsor to the contract address. State transitions to `Funded`.

### 3. Care Attestation (`attest_care`)
- **Caller**: Designated attester (`attester.require_auth()`).
- **Prerequisites**: Agreement must be in `Funded` state and current ledger timestamp must be `<= care_deadline`.
- **Execution**: Records `attestation_commitment`, timestamp, and transitions state to `CareConfirmed`.

### 4. Settlement (`settle`)
- **Caller**: Anyone (permissionless deterministic finalization).
- **Prerequisites**: Agreement must be in `CareConfirmed` state and current ledger timestamp must be `> care_deadline + dispute_window_secs` (dispute window has elapsed).
- **Execution**: Transfers `settlement_amount` from contract to `provider`, and atomically refunds any surplus (`funding_amount - settlement_amount`) to `sponsor`. State transitions to `Settled`. Contract escrow balance drops to zero.

### 5. Disputes (`open_dispute` & `resolve_dispute`)
- **Opening**: Either sponsor or provider can invoke `open_dispute` during the active dispute window (`care_deadline < now <= care_deadline + dispute_window_secs`).
- **Resolution**: Contract admin invokes `resolve_dispute`:
  - `DisputeResolution::Resume`: Restores original pre-dispute state.
  - `DisputeResolution::Settle`: Disburses `settlement_amount` to provider, refunds surplus (`funding_amount - settlement_amount`) to sponsor, and marks agreement `Settled`.
  - `DisputeResolution::Refund`: Returns full deposited `funding_amount` to sponsor and marks agreement `Refunded`.

### 6. Expiration and Cancellation
- **`cancel`**: Provider can cancel an un-funded `Requested` agreement.
- **`expire`**: Anyone can trigger expiration if deadlines lapse:
  - If in `Requested` after `funding_deadline`: state becomes `Expired` with zero token transfers.
  - If in `Funded` after `care_deadline + dispute_window_secs` without attestation: state becomes `Expired` and the full deposited escrow is automatically refunded to sponsor.
  - Agreements in `CareConfirmed` cannot be expired (`InvalidState`).
