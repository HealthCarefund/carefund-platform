import { apiBaseUrl } from "./stellar-config";

// These mirror apps/api/internal/api's JSON response shapes exactly
// (field-for-field) — see providers.go, agreements.go, intents.go,
// transactions.go, transactionlookup.go. Every u64/i128/amount field stays
// a string end to end, never a JS number.

export interface ProviderResponse {
  walletAddress: string;
  providerRef: string;
  status: "Active" | "Suspended" | "Revoked";
  updatedAt: string;
}

export interface AttesterResponse {
  walletAddress: string;
  providerWallet: string;
  credentialRef: string;
  status: "Active" | "Suspended" | "Revoked";
  updatedAt: string;
}

export type AgreementState =
  | "Requested"
  | "Funded"
  | "CareConfirmed"
  | "Disputed"
  | "Cancelled"
  | "Expired"
  | "Refunded"
  | "Settled";

export interface AgreementResponse {
  agreementId: string;
  sponsorWallet: string;
  providerWallet: string;
  attesterWallet: string;
  patientRefCommitment: string;
  serviceCommitment: string;
  fundingAmount: string;
  settlementAmount: string;
  settlementAssetContractId: string;
  fundingDeadline: string;
  careDeadline: string;
  disputeWindowSecs: string;
  state: AgreementState;
  updatedAt: string;
}

export interface ContractEventResponse {
  id: string;
  contractId: string;
  txHash: string;
  eventIndex: number;
  eventType: string;
  agreementId?: string;
  ledger: string;
  observedAt: string;
}

export interface EventsPageResponse {
  events: ContractEventResponse[];
  nextCursor?: string;
}

export interface IntentResponse {
  id: string;
  sponsorWallet: string;
  providerWallet: string;
  attesterWallet: string;
  patientRefCommitment: string;
  serviceCommitment: string;
  fundingAmount: string;
  settlementAmount: string;
  settlementAssetContractId: string;
  fundingDeadline: string;
  careDeadline: string;
  disputeWindowSecs: string;
  status: "pending" | "submitted" | "expired" | "cancelled";
  agreementId?: string;
  createdAt: string;
}

export interface CreateIntentRequest {
  sponsorWallet: string;
  providerWallet: string;
  attesterWallet: string;
  patientRefCommitment: string;
  serviceCommitment: string;
  fundingAmount: string;
  settlementAmount: string;
  settlementAssetContractId: string;
  fundingDeadline: string;
  careDeadline: string;
  disputeWindowSecs: string;
}

export type PrepareTransactionOperation =
  | "fund"
  | "cancel"
  | "attest_care"
  | "open_dispute"
  | "expire"
  | "settle"
  | "resolve_dispute";

export interface PrepareTransactionRequest {
  operation: PrepareTransactionOperation;
  sourcePublicKey: string;
  attestationCommitment?: string;
  resolution?: "Resume" | "Settle" | "Refund";
}

export interface PrepareTransactionResponse {
  unsignedTransactionXdr: string;
  network: string;
  networkPassphrase: string;
}

export type TransactionLookupStatus = "success" | "failed" | "not_found";

export interface ReconciliationInfo {
  operation: string;
  agreementId?: string;
  firstSeen: string;
  lastChecked?: string;
  confirmedAt?: string;
  errorCode?: string;
  errorDetail?: string;
}

export interface TransactionLookupResponse {
  hash: string;
  status: TransactionLookupStatus;
  ledger?: string;
  reconciliation?: ReconciliationInfo;
}

export interface ApiErrorFieldIssue {
  field: string;
  issue: string;
}

export interface ApiErrorBody {
  kind: "validation_error" | "not_found" | "conflict" | "contract_error" | "internal_error";
  message: string;
  fields?: ApiErrorFieldIssue[];
}

export class ApiError extends Error {
  constructor(readonly status: number, readonly body: ApiErrorBody) {
    super(body.message);
    this.name = "ApiError";
  }
}

function newIdempotencyKey(): string {
  return crypto.randomUUID();
}

async function request<T>(
  path: string,
  init?: RequestInit & { idempotent?: boolean },
): Promise<T> {
  const headers = new Headers(init?.headers);
  if (init?.body) headers.set("Content-Type", "application/json");
  if (init?.idempotent) headers.set("Idempotency-Key", newIdempotencyKey());

  const response = await fetch(`${apiBaseUrl()}${path}`, { ...init, headers });
  const text = await response.text();
  const json = text ? JSON.parse(text) : undefined;

  if (!response.ok) {
    throw new ApiError(response.status, json as ApiErrorBody);
  }
  return json as T;
}

export function getProvider(wallet: string): Promise<ProviderResponse> {
  return request(`/api/v1/providers/${encodeURIComponent(wallet)}`);
}

export function listProviderAttesters(wallet: string): Promise<AttesterResponse[]> {
  return request(`/api/v1/providers/${encodeURIComponent(wallet)}/attesters`);
}

export interface ProvidersPageResponse {
  providers: ProviderResponse[];
  nextCursor?: string;
}

export interface AttestersPageResponse {
  attesters: AttesterResponse[];
  nextCursor?: string;
}

export function listProviders(options?: { cursor?: string; limit?: number }): Promise<ProvidersPageResponse> {
  const params = new URLSearchParams();
  if (options?.cursor) params.set("cursor", options.cursor);
  if (options?.limit) params.set("limit", String(options.limit));
  const query = params.toString() ? `?${params.toString()}` : "";
  return request(`/api/v1/providers${query}`);
}

export function listAttesters(options?: { cursor?: string; limit?: number }): Promise<AttestersPageResponse> {
  const params = new URLSearchParams();
  if (options?.cursor) params.set("cursor", options.cursor);
  if (options?.limit) params.set("limit", String(options.limit));
  const query = params.toString() ? `?${params.toString()}` : "";
  return request(`/api/v1/attesters${query}`);
}

export interface AgreementsPageResponse {
  agreements: AgreementResponse[];
  nextCursor?: string;
}

export function listProviderAgreements(
  wallet: string,
  options?: { cursor?: string; limit?: number },
): Promise<AgreementsPageResponse> {
  const params = new URLSearchParams();
  if (options?.cursor) params.set("cursor", options.cursor);
  if (options?.limit) params.set("limit", String(options.limit));
  const query = params.toString() ? `?${params.toString()}` : "";
  return request(`/api/v1/providers/${encodeURIComponent(wallet)}/agreements${query}`);
}

export function listSponsorAgreements(
  wallet: string,
  options?: { cursor?: string; limit?: number },
): Promise<AgreementsPageResponse> {
  const params = new URLSearchParams();
  if (options?.cursor) params.set("cursor", options.cursor);
  if (options?.limit) params.set("limit", String(options.limit));
  const query = params.toString() ? `?${params.toString()}` : "";
  return request(`/api/v1/sponsors/${encodeURIComponent(wallet)}/agreements${query}`);
}

export function getAgreement(agreementId: string): Promise<AgreementResponse> {
  return request(`/api/v1/agreements/${encodeURIComponent(agreementId)}`);
}

export function getAgreementEvents(
  agreementId: string,
  options?: { cursor?: string; limit?: number },
): Promise<EventsPageResponse> {
  const params = new URLSearchParams();
  if (options?.cursor) params.set("cursor", options.cursor);
  if (options?.limit) params.set("limit", String(options.limit));
  const query = params.toString() ? `?${params.toString()}` : "";
  return request(`/api/v1/agreements/${encodeURIComponent(agreementId)}/events${query}`);
}

export function createAgreementIntent(body: CreateIntentRequest): Promise<IntentResponse> {
  return request("/api/v1/agreements/intents", {
    method: "POST",
    body: JSON.stringify(body),
    idempotent: true,
  });
}

export function prepareAgreementTransaction(
  agreementId: string,
  body: PrepareTransactionRequest,
): Promise<PrepareTransactionResponse> {
  return request(`/api/v1/agreements/${encodeURIComponent(agreementId)}/transactions`, {
    method: "POST",
    body: JSON.stringify(body),
    idempotent: true,
  });
}

export function lookupTransaction(hash: string): Promise<TransactionLookupResponse> {
  return request(`/api/v1/transactions/${encodeURIComponent(hash)}`);
}
