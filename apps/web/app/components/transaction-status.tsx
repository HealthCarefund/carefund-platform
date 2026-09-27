import type { TransactionFlowState } from "@/lib/use-transaction-flow";

const STAGE_LABELS: Record<TransactionFlowState["phase"], string> = {
  idle: "Not started",
  preparing: "Preparing transaction",
  simulating: "Simulating on-chain",
  awaiting_signature: "Waiting for wallet signature",
  submitting: "Submitting to the network",
  submitted: "Submitted — waiting for confirmation",
  confirming: "Confirming",
  confirmed: "Confirmed",
  failed: "Failed",
  timeout: "Confirmation timed out",
};

const IN_PROGRESS_PHASES = new Set<TransactionFlowState["phase"]>([
  "preparing",
  "simulating",
  "awaiting_signature",
  "submitting",
  "submitted",
  "confirming",
]);

function toneFor(phase: TransactionFlowState["phase"]): "neutral" | "progress" | "success" | "danger" {
  if (phase === "confirmed") return "success";
  if (phase === "failed" || phase === "timeout") return "danger";
  if (IN_PROGRESS_PHASES.has(phase)) return "progress";
  return "neutral";
}

const TONE_CLASSES: Record<ReturnType<typeof toneFor>, string> = {
  neutral: "border-[var(--border-subtle)] text-ink-500 dark:text-ink-400",
  progress: "border-brand-300 text-brand-700 dark:border-brand-700 dark:text-brand-300",
  success: "border-success-500 text-success-600",
  danger: "border-danger-500 text-danger-500",
};

/**
 * Renders exactly one of this flow's distinct stages at a time.
 * "submitted" and "confirmed" are deliberately different rows with
 * different copy — a submission being accepted by the network is not the
 * same thing as it having landed, and this component never blurs that
 * distinction into a single "success"-looking state.
 */
export function TransactionStatus({ state }: { state: TransactionFlowState }) {
  const tone = toneFor(state.phase);
  const label = STAGE_LABELS[state.phase];

  return (
    <div
      role="status"
      aria-live="polite"
      className={`rounded-lg border px-4 py-3 text-sm ${TONE_CLASSES[tone]}`}
    >
      <p className="font-medium">{label}</p>

      {"hash" in state && state.hash && (
        <p className="mt-1 font-mono text-xs break-all text-ink-500 dark:text-ink-400">
          Transaction: {state.hash}
        </p>
      )}
      {state.phase === "confirming" && (
        <p className="mt-1 text-xs text-ink-500 dark:text-ink-400">
          Checking attempt {state.attempt} — not yet confirmed.
        </p>
      )}
      {state.phase === "confirmed" && state.ledger && (
        <p className="mt-1 text-xs text-ink-500 dark:text-ink-400">Ledger {state.ledger}</p>
      )}
      {state.phase === "failed" && (
        <p className="mt-1 text-xs">{state.reason}</p>
      )}
      {state.phase === "timeout" && (
        <p className="mt-1 text-xs text-ink-500 dark:text-ink-400">
          The network hasn&rsquo;t confirmed this yet. It may still land — check the transaction
          hash again shortly rather than resubmitting.
        </p>
      )}
    </div>
  );
}
