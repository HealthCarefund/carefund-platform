// Command reconciliation-recovery is a local operator maintenance command.
// It is intentionally not mounted by the API server and never submits a
// transaction or edits reconciliation rows directly.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/user"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/HealthCarefund/carefund-platform/apps/api/internal/config"
	"github.com/HealthCarefund/carefund-platform/apps/api/internal/reconcile"
	"github.com/HealthCarefund/carefund-platform/apps/api/internal/stellarrpc"
	"github.com/HealthCarefund/carefund-platform/apps/api/internal/store"
)

func main() {
	if len(os.Args) != 2 && (len(os.Args) < 2 || (os.Args[1] != "acknowledge")) {
		usage()
		os.Exit(2)
	}

	command := os.Args[1]
	cfg, err := config.Load(os.LookupEnv)
	if err != nil {
		fail(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.RequestTimeout)
	defer cancel()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		fail(fmt.Errorf("opening database: %w", err))
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		fail(fmt.Errorf("checking database: %w", err))
	}

	rpc := stellarrpc.New(cfg.StellarRPCURL, cfg.RPCTimeout)
	defer rpc.Close()
	runner := reconcile.New(store.New(pool), rpc, slog.New(slog.NewTextHandler(os.Stderr, nil)), cfg.ReconciliationInterval, cfg.ProviderRegistryContractID, cfg.CareAgreementContractID, cfg.SettlementAssetContractID)

	switch command {
	case "inspect":
		if err := inspect(ctx, runner); err != nil {
			fail(err)
		}
	case "acknowledge":
		if err := acknowledge(ctx, runner, os.Args[2:]); err != nil {
			fail(err)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func inspect(ctx context.Context, runner *reconcile.Runner) error {
	snapshot, err := runner.InspectEventRecovery(ctx)
	if err != nil {
		return err
	}
	printSnapshot(snapshot)
	return nil
}

type acknowledgeOptions struct {
	confirmedGap string
	resumeLedger uint32
}

func parseAcknowledgeOptions(args []string) (acknowledgeOptions, error) {
	flags := flag.NewFlagSet("acknowledge", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	confirmedGap := flags.String("confirm-gap", "", "exact gap ID printed by inspect")
	resumeLedgerRaw := flags.String("resume-ledger", "", "exact proposed resume ledger printed by inspect")
	yes := flags.Bool("yes", false, "confirm acceptance of the displayed missing range")
	if err := flags.Parse(args); err != nil {
		return acknowledgeOptions{}, err
	}
	if flags.NArg() != 0 {
		return acknowledgeOptions{}, fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}
	if !*yes || *confirmedGap == "" {
		return acknowledgeOptions{}, errors.New("acknowledge requires --confirm-gap, --resume-ledger, and --yes")
	}
	resumeLedger, err := parseResumeLedger(*resumeLedgerRaw)
	if err != nil {
		return acknowledgeOptions{}, err
	}
	return acknowledgeOptions{confirmedGap: *confirmedGap, resumeLedger: resumeLedger}, nil
}

func parseResumeLedger(raw string) (uint32, error) {
	if raw == "" {
		return 0, errors.New("--resume-ledger is required")
	}
	for _, char := range raw {
		if char < '0' || char > '9' {
			return 0, fmt.Errorf("--resume-ledger %q must be a base-10 unsigned integer", raw)
		}
	}
	value, err := strconv.ParseUint(raw, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("--resume-ledger %q is outside the uint32 ledger range", raw)
	}
	if value == 0 {
		return 0, errors.New("--resume-ledger must be greater than zero")
	}
	return uint32(value), nil
}

func acknowledge(ctx context.Context, runner *reconcile.Runner, args []string) error {
	opts, err := parseAcknowledgeOptions(args)
	if err != nil {
		return err
	}

	snapshot, err := runner.InspectEventRecovery(ctx)
	if err != nil {
		return err
	}
	printSnapshot(snapshot)
	if snapshot.GapError != "" {
		return fmt.Errorf("gap data is malformed: %s", snapshot.GapError)
	}
	if snapshot.Gap == nil {
		return errors.New("no valid retention gap is pending")
	}
	if snapshot.Gap.AcknowledgedAt != "" {
		return errors.New("retention gap was already acknowledged")
	}
	if snapshot.RPCError != "" {
		return fmt.Errorf("cannot confirm resume position while RPC health is unavailable: %s", snapshot.RPCError)
	}
	if opts.confirmedGap != snapshot.GapID {
		return fmt.Errorf("--confirm-gap does not match the inspected gap")
	}
	if opts.resumeLedger != snapshot.RPCOldestLedger {
		return fmt.Errorf("--resume-ledger %d does not match the current proposed resume ledger %d", opts.resumeLedger, snapshot.RPCOldestLedger)
	}
	skipped, err := reconcile.CalculateCompleteSkippedRange(*snapshot.Gap, snapshot.RPCOldestLedger, opts.resumeLedger)
	if err != nil {
		return err
	}

	operator, err := currentOperator()
	if err != nil {
		return err
	}
	metadata, err := json.Marshal(map[string]any{
		"operator":                        operator,
		"command":                         "reconciliation-recovery acknowledge",
		"gap_id":                          snapshot.GapID,
		"original_gap_from_ledger":        snapshot.Gap.RequestedLedger,
		"original_gap_through_ledger":     skipped.OriginalThroughLedger,
		"current_rpc_oldest_ledger":       skipped.CurrentRPCOldestLedger,
		"accepted_missing_from_ledger":    skipped.FromLedger,
		"accepted_missing_through_ledger": skipped.ThroughLedger,
		"approved_resume_ledger":          skipped.ApprovedResumeLedger,
		"historical_events_recovered":     false,
	})
	if err != nil {
		return fmt.Errorf("encoding recovery audit: %w", err)
	}
	if err := runner.AcknowledgeEventRetentionGapAt(ctx, snapshot.GapID, snapshot.EventCursor, snapshot.RPCOldestLedger, store.ReconciliationAudit{
		Action:     "acknowledge_event_retention_gap",
		ResourceID: "contract_events",
		Metadata:   metadata,
	}); err != nil {
		return err
	}
	fmt.Printf("accepted missing historical range %d-%d; resumed from ledger %d; events were not recovered\n", skipped.FromLedger, skipped.ThroughLedger, skipped.ApprovedResumeLedger)
	return nil
}

func printSnapshot(snapshot reconcile.EventRecoverySnapshot) {
	fmt.Printf("Event cursor: %s\n", display(snapshot.EventCursor))
	fmt.Printf("RPC ledgers: oldest=%s latest=%s\n", displayUint(snapshot.RPCOldestLedger), displayUint(snapshot.RPCLatestLedger))
	fmt.Printf("Reconciliation blocked: %t\n", snapshot.ReconciliationBlocked)
	fmt.Printf("Historical range status: %s\n", snapshot.HistoricalRangeStatus)
	fmt.Printf("RPC retention may permit replay: %t\n", snapshot.HistoricalRecoveryPossible)
	fmt.Printf("Historical event completeness independently verified: %t\n", snapshot.HistoricalCompletenessVerified)
	if snapshot.RPCError != "" {
		fmt.Printf("RPC health: unavailable (%s)\n", snapshot.RPCError)
	}
	if snapshot.Gap == nil {
		if snapshot.GapError != "" {
			fmt.Printf("Retention gap: malformed (id=%s): %s\n", snapshot.GapID, snapshot.GapError)
		} else {
			fmt.Println("Retention gap: none")
		}
	} else {
		fmt.Printf("Retention gap ID: %s\n", snapshot.GapID)
		fmt.Printf("Original gap recorded: %d-%d\n", snapshot.Gap.RequestedLedger, snapshot.Gap.RPCOldestLedger-1)
		if snapshot.Gap.AcknowledgedAt != "" {
			fmt.Printf("Complete range accepted at acknowledgment: %d-%d\n", snapshot.Gap.RequestedLedger, snapshot.Gap.ResumedAtLedger-1)
			fmt.Printf("Approved resume ledger: %d\n", snapshot.Gap.ResumedAtLedger)
		} else if snapshot.RPCOldestLedger > snapshot.Gap.RequestedLedger {
			fmt.Printf("Complete range that would be accepted now: %d-%d\n", snapshot.Gap.RequestedLedger, snapshot.RPCOldestLedger-1)
			fmt.Printf("Proposed resume ledger: %d\n", snapshot.RPCOldestLedger)
		} else {
			fmt.Println("Complete range that would be accepted now: unavailable")
		}
		fmt.Printf("Acknowledged: %t\n", snapshot.Gap.AcknowledgedAt != "")
	}
	fmt.Printf("Previous recovery decisions: %d\n", len(snapshot.PreviousRecoveryDecisions))
	for _, decision := range snapshot.PreviousRecoveryDecisions {
		fmt.Printf("  #%d %s %s at %s\n", decision.ID, decision.Action, decision.Result, decision.CreatedAt.Format(time.RFC3339))
	}
}

func currentOperator() (string, error) {
	current, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("determining operating-system user: %w", err)
	}
	host, err := os.Hostname()
	if err != nil {
		return "", fmt.Errorf("determining host: %w", err)
	}
	return current.Username + "@" + host, nil
}

func display(value string) string {
	if value == "" {
		return "<none>"
	}
	return value
}

func displayUint(value uint32) string {
	if value == 0 {
		return "<unavailable>"
	}
	return strconv.FormatUint(uint64(value), 10)
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: reconciliation-recovery inspect")
	fmt.Fprintln(os.Stderr, "       reconciliation-recovery acknowledge --confirm-gap ID --resume-ledger N --yes")
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "reconciliation-recovery:", err)
	os.Exit(1)
}
