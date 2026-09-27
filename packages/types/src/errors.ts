/** Mirrors `RegistryError` in contracts/provider-registry/src/lib.rs exactly. */
export type RegistryErrorCode =
  | "AlreadyInitialized"
  | "Unauthorized"
  | "ProviderNotFound"
  | "ProviderAlreadyExists"
  | "ProviderAlreadySuspended"
  | "ProviderAlreadyActive"
  | "ProviderAlreadyRevoked"
  | "AttesterNotFound"
  | "AttesterAlreadyExists"
  | "AttesterAlreadySuspended"
  | "AttesterAlreadyActive"
  | "AttesterAlreadyRevoked"
  | "ProviderNotActive"
  | "InvalidState";

/** Mirrors `AgreementError` in contracts/care-agreement/src/lib.rs exactly. */
export type AgreementErrorCode =
  | "AlreadyInitialized"
  | "Unauthorized"
  | "AgreementNotFound"
  | "InvalidState"
  | "InvalidAmount"
  | "InvalidDeadline"
  | "ProviderNotActive"
  | "AttesterNotAuthorized"
  | "FundingDeadlineNotReached"
  | "FundingDeadlineNotPassed"
  | "CareDeadlineNotPassed"
  | "DisputeWindowActive"
  | "DisputeWindowClosed"
  | "InvalidResolution"
  | "TransferFailed"
  | "ArithmeticOverflow";

export type ContractErrorCode = RegistryErrorCode | AgreementErrorCode;

export type ContractName = "provider-registry" | "care-agreement";

/**
 * A contract call failed and surfaced one of the exact on-chain error
 * codes above.
 */
export interface ContractErrorDetail {
  readonly contract: ContractName;
  readonly code: ContractErrorCode;
}

/**
 * API error response shape. `kind` discriminates the cause; `contract_error`
 * carries the exact on-chain error code so clients never have to guess
 * what a generic message meant.
 */
export type ApiErrorShape =
  | {
      readonly kind: "contract_error";
      readonly message: string;
      readonly detail: ContractErrorDetail;
    }
  | {
      readonly kind: "validation_error";
      readonly message: string;
      readonly fields: ReadonlyArray<{ readonly field: string; readonly issue: string }>;
    }
  | {
      readonly kind: "not_found";
      readonly message: string;
    }
  | {
      readonly kind: "conflict";
      readonly message: string;
    }
  | {
      readonly kind: "internal_error";
      readonly message: string;
    };
