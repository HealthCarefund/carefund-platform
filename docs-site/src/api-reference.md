# API Reference

The CareFund Go backend (`apps/api`) provides REST endpoints for off-chain workflow coordination, unsigned transaction construction, and ledger event queries.

All API routes are prefixed with `/api/v1/` (except health check endpoints).

## Health and Readiness

### `GET /healthz`
Liveness probe returning HTTP 200 if the HTTP daemon is running.
- **Response**: `{"status": "ok"}`

### `GET /readyz`
Readiness probe verifying database connectivity against PostgreSQL 18.6.
- **Response**: `{"status": "ready"}`

---

## Providers and Attesters

### `GET /api/v1/providers`
Returns a list of registered healthcare providers.
- **Query Parameters**:
  - `status` (optional): Filter by `active`, `suspended`, or `revoked`.
- **Response**:
  ```json
  [
    {
      "wallet": "GARYN44ZRCQYNXHR6VEB6ZLJWAMSF7SAO6CDCM6NQHOXMCSKJ54UCJBP",
      "status": "active",
      "providerRef": "a1b2c3d4...",
      "createdAt": "2026-09-27T10:00:00Z"
    }
  ]
  ```

### `GET /api/v1/attesters`
Returns a list of authorized attesters across providers.

### `GET /api/v1/providers/{wallet}`
Returns details for a specific provider wallet.

### `GET /api/v1/providers/{wallet}/attesters`
Returns attesters specifically linked to the given provider wallet.

---

## Agreements and Intents

### `GET /api/v1/agreements/{agreementId}`
Fetches the cached agreement state and metadata for an agreement.
- **Response**:
  ```json
  {
    "id": 1,
    "sponsor": "GDEI32FXSM...",
    "provider": "GARYN44Z...",
    "attester": "GBD5ORGL...",
    "fundingAmount": "50000000",
    "settlementAmount": "40000000",
    "state": "CareConfirmed",
    "fundingDeadline": 1790500000,
    "careDeadline": 1790500060,
    "disputeWindowSecs": 60
  }
  ```

### `GET /api/v1/agreements/{agreementId}/events`
Returns the chronological list of on-chain ledger events ingested for this agreement.

### `GET /api/v1/providers/{wallet}/agreements`
Lists all agreements where the given wallet is the provider.

### `GET /api/v1/sponsors/{wallet}/agreements`
Lists all agreements where the given wallet is the sponsor.

### `POST /api/v1/agreements/intents`
Creates an off-chain record of an agreement intent before on-chain creation.
- **Headers**: `Idempotency-Key: <unique-uuid>` (Required)
- **Request Body**:
  ```json
  {
    "sponsorWallet": "GDEI32FX...",
    "providerWallet": "GARYN44Z...",
    "attesterWallet": "GBD5ORGL...",
    "patientRefCommitment": "hex-encoded-32-byte-hash",
    "serviceCommitment": "hex-encoded-32-byte-hash",
    "fundingAmount": "50000000",
    "settlementAmount": "40000000",
    "fundingDurationSecs": 86400,
    "careDurationSecs": 172800,
    "disputeWindowSecs": 604800
  }
  ```
- **Response**: `{"id": "intent-uuid", "status": "created"}`

---

## Transaction Preparation and Tracking

### `POST /api/v1/agreements/{agreementId}/transactions`
Prepares, simulates, and returns unsigned transaction XDR for an agreement lifecycle operation.
- **Headers**: `Idempotency-Key: <unique-uuid>` (Required)
- **Request Body**:
  ```json
  {
    "operation": "fund",
    "caller": "GDEI32FXSM6XIOKHUXT43MOB7GTKRRZVBNVVCW6XE6DXXKL7J3LKCI3S"
  }
  ```
  Supported operations: `fund`, `cancel`, `attest_care`, `open_dispute`, `expire`, `settle`, `resolve_dispute`.
- **Response**:
  ```json
  {
    "unsignedXdr": "AAAAAgAAAA...",
    "simulatedFee": "1000",
    "operation": "fund"
  }
  ```

### `GET /api/v1/transactions/{hash}`
Queries the submission status of a transaction hash known to the API and reconciliation worker.
- **Response**:
  ```json
  {
    "hash": "833b4c6e554557317c9afae3a9862321931259565a7ff2917877f062927b6951",
    "status": "confirmed",
    "ledger": 123456,
    "confirmedAt": "2026-09-27T10:15:00Z"
  }
  ```
