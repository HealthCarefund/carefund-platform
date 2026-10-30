# Environment Configuration

CareFund enforces strict cross-validation on all network and configuration variables at startup. Mismatched or missing configuration causes services to fail immediately rather than directing signed transactions to incorrect networks.

## Go API Configuration (`apps/api/.env`)

The Go API entry point calls `config.Load(os.LookupEnv)`. It reads process environment variables and does not parse `.env` files automatically. For local development, copy the checked-in example and source it in the shell that starts the API:

```bash
cp apps/api/.env.example apps/api/.env
set -a
source apps/api/.env
set +a
```

The copied file is ignored by Git. It contains local PostgreSQL settings and public Block 3B Testnet contract parameters; it contains no signing key.

| Variable | Required | Default / Example | Purpose |
|---|---|---|---|
| `APP_ENV` | Yes | `development` | Runtime environment label |
| `APP_ADDR` | Yes | `:8080` | HTTP listening address |
| `DATABASE_URL` | Yes | `postgres://carefund:carefund_dev@localhost:5439/carefund?sslmode=disable` | Connection string for PostgreSQL 18.6 |
| `STELLAR_NETWORK` | Yes | `TESTNET` | Network identifier (`TESTNET` or `CUSTOM`) |
| `STELLAR_RPC_URL` | Yes | `https://soroban-testnet.stellar.org` | Soroban JSON-RPC endpoint |
| `STELLAR_NETWORK_PASSPHRASE` | Yes | `Test SDF Network ; September 2015` | Network cryptographic passphrase |
| `PROVIDER_REGISTRY_CONTRACT_ID` | Yes | `CCY5673G6KNI6JRRRZ46NKQU7HVCA4G4V7XH3YMZIVGQ7S7HBWDDQ7ZS` | Deployed address of provider-registry |
| `CARE_AGREEMENT_CONTRACT_ID` | Yes | `CCBBYEVOXW2BS4V7OGRD63E3UU2Y77RF25DGBYGTZ3RFKLZTMPYNZQ4O` | Deployed address of care-agreement |
| `SETTLEMENT_ASSET_CONTRACT_ID` | Yes | `CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC` | Deployed address of settlement asset (native SAC) |
| `RPC_TIMEOUT` | Yes | `5s` | Soroban RPC request timeout |
| `REQUEST_TIMEOUT` | Yes | `10s` | HTTP request timeout |
| `RECONCILIATION_INTERVAL` | Yes | `30s` | Background reconciliation interval |
| `CORS_ALLOWED_ORIGINS` | Yes | `http://localhost:3000` | Allowed browser origins, comma-separated |
| `LOG_LEVEL` | Yes | `info` | Structured log level |
| `MAX_REQUEST_BODY_BYTES` | Yes | `1048576` | Maximum accepted request body size |

`SKIP_LIVE_NETWORK_TESTS=1` is a test-only CI variable, not an API runtime setting. Set it when running the Go tests without outbound Testnet access.

---

## Next.js Frontend Configuration (`apps/web/.env.local`)

All frontend variables use the `NEXT_PUBLIC_` prefix because they contain public routing and contract parameters required by the browser:

| Variable | Required | Default / Example | Purpose |
|---|---|---|---|
| `NEXT_PUBLIC_STELLAR_NETWORK` | Yes | `TESTNET` | Informs SDK which network to validate against |
| `NEXT_PUBLIC_STELLAR_RPC_URL` | Yes | `https://soroban-testnet.stellar.org` | RPC endpoint used for simulation and submission |
| `NEXT_PUBLIC_STELLAR_NETWORK_PASSPHRASE` | Yes | `Test SDF Network ; September 2015` | Required for signing envelopes |
| `NEXT_PUBLIC_PROVIDER_REGISTRY_CONTRACT_ID` | Yes | `CCY5673...` | Address for provider queries |
| `NEXT_PUBLIC_CARE_AGREEMENT_CONTRACT_ID` | Yes | `CCBBYEV...` | Address for agreement lifecycle actions |
| `NEXT_PUBLIC_SETTLEMENT_ASSET_CONTRACT_ID` | Yes | `CDLZFC...` | Address for settlement asset |
| `NEXT_PUBLIC_API_BASE_URL` | Yes | `http://localhost:8080` | URL for the Go API service |

---

## Network Validation and Fail-Safe Rules

In `@carefund/sdk/src/config.ts`, the function `validateStellarClientConfig` enforces strict invariant checks:

1. **Passphrase Matching**:
   If `STELLAR_NETWORK` is set to `TESTNET`, the passphrase must exactly match `Test SDF Network ; September 2015`. Any deviation triggers a startup error.
2. **Contract ID Encoding**:
   Every contract ID must begin with `C` and decode to a valid 32-byte Stellar StrKey address.
3. **No Silent Fallbacks**:
   The application never falls back to default networks or unauthenticated public endpoints without configuration.
