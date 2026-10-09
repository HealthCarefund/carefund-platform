# Troubleshooting Guide

This guide covers common issues encountered during local development, toolchain configuration, and testing.

## Database and Docker Issues

### Connection Refused on Port 5439
- **Symptom**: `dial tcp 127.0.0.1:5439: connect: connection refused`.
- **Cause**: The isolated Docker container is not running.
- **Solution**:
  ```bash
  docker compose up -d
  docker compose ps
  ```

### Database Port Conflict
- **Symptom**: `bind: address already in use` when starting Docker.
- **Cause**: Another service is already listening on host port 5439.
- **Solution**: Stop conflicting local processes or containers bound to port 5439 before starting `carefund-pg18`.

---

## Testing and Toolchain Issues

### Go Tests Intermittently Fail
- **Symptom**: `pq: relation does not exist` or foreign key violations during `go test ./...`.
- **Cause**: Running Go tests in parallel across packages causes concurrent table truncation in the database.
- **Solution**: Always run Go tests with serialized package execution:
  ```bash
  cd apps/api
  go test -p 1 ./...
  ```

### Node or pnpm Version Mismatch
- **Symptom**: Corepack prompt loops or engine warning during install.
- **Cause**: Active Node.js runtime does not match `.nvmrc` (v24.21.0).
- **Solution**:
  ```bash
  nvm use
  corepack enable
  corepack prepare pnpm@12.5.1 --activate
  ```

### Rust Contract Target Missing
- **Symptom**: `error[E0463]: can't find crate for core` when running `stellar contract build`.
- **Cause**: The WebAssembly target is not installed in the active toolchain.
- **Solution**:
  ```bash
  rustup target add wasm32v1-none
  ```

---

## Wallet and Client Issues

### Wrong Network Error in Browser
- **Symptom**: `@carefund/sdk` throws `WrongNetworkError` upon wallet connection.
- **Cause**: Freighter wallet extension is pointed at Mainnet, Public Futurenet, or a standalone network instead of Stellar Testnet.
- **Solution**: In the Freighter browser extension settings, switch active network to `Testnet` and confirm passphrase is `Test SDF Network ; September 2015`.

### API Returns HTTP 409 Conflict
- **Symptom**: Mutating POST request fails with status 409.
- **Cause**: Reusing an existing `Idempotency-Key` header with a different request payload body.
- **Solution**: Generate a fresh UUID for distinct requests, or supply the exact original payload to replay the cached response.
