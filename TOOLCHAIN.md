# Toolchain

CareFund pins exact versions of every tool. Run `scripts/check-toolchain.sh`
to verify your local environment matches.

| Tool       | Version    | Selected by                                   |
|------------|------------|------------------------------------------------|
| Go         | 1.27.1     | `go.work`, `apps/api/go.mod` (`go 1.27.1`)     |
| Node.js    | 24.21.0    | `.nvmrc`, `package.json` (`engines.node`)      |
| pnpm       | 12.5.1     | `package.json` (`packageManager`), via Corepack |
| Rust       | 1.98.1     | `rust-toolchain.toml`                          |
| PostgreSQL | 18.6       | `docker-compose.yml` (`postgres:18.6-bookworm`, isolated container `carefund-pg18`, host port 5439) |
| Stellar CLI| 27.0.0     | Installed via `cargo install --locked stellar-cli --version 27.0.0`; verified by `scripts/check-toolchain.sh` |

The host's system Go (1.25.1) and PostgreSQL 16 installation are left
untouched; CareFund never depends on them.
