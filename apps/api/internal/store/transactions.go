package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// InsertTransactionRef records a newly submitted transaction. tx_hash is
// unique: a duplicate insert attempt (the same hash submitted twice) is a
// caller bug, not a normal case, and surfaces as a plain error rather than
// silently upserting — reconciliation updates status via
// UpdateTransactionStatus, never by re-inserting.
func (s *Store) InsertTransactionRef(ctx context.Context, agreementID *int64, operation, txHash, status string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO transaction_refs (agreement_id, operation, tx_hash, status)
		VALUES ($1, $2, $3, $4)
	`, agreementID, operation, txHash, status)
	if err != nil {
		return fmt.Errorf("inserting transaction ref %q: %w", txHash, err)
	}
	return nil
}

// UpdateTransactionStatus reconciles a transaction's status against the
// chain. ledger/confirmedAt/errorCode/errorDetail are nil unless known.
// last_checked always advances, so every reconciliation attempt is
// visible even when the status hasn't changed.
func (s *Store) UpdateTransactionStatus(
	ctx context.Context,
	txHash, status string,
	ledger *int64,
	confirmedAt *time.Time,
	errorCode, errorDetail *string,
) error {
	cmd, err := s.pool.Exec(ctx, `
		UPDATE transaction_refs
		SET status = $2,
		    last_checked = now(),
		    ledger = COALESCE($3, ledger),
		    confirmed_at = COALESCE($4, confirmed_at),
		    error_code = COALESCE($5, error_code),
		    error_detail = COALESCE($6, error_detail)
		WHERE tx_hash = $1
	`, txHash, status, ledger, confirmedAt, errorCode, errorDetail)
	if err != nil {
		return fmt.Errorf("updating transaction ref %q: %w", txHash, err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetTransactionByHash returns ErrNotFound if no ref with that hash exists.
func (s *Store) GetTransactionByHash(ctx context.Context, txHash string) (*TransactionRef, error) {
	var t TransactionRef
	err := s.pool.QueryRow(ctx, `
		SELECT id, agreement_id, operation, tx_hash, status, first_seen, last_checked, confirmed_at, ledger, error_code, error_detail
		FROM transaction_refs
		WHERE tx_hash = $1
	`, txHash).Scan(
		&t.ID, &t.AgreementID, &t.Operation, &t.TxHash, &t.Status,
		&t.FirstSeen, &t.LastChecked, &t.ConfirmedAt, &t.Ledger, &t.ErrorCode, &t.ErrorDetail,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting transaction ref %q: %w", txHash, err)
	}
	return &t, nil
}

// ListPendingTransactions returns every transaction ref not yet in a
// terminal state ("confirmed" or "failed"), for reconciliation to poll.
func (s *Store) ListPendingTransactions(ctx context.Context) ([]TransactionRef, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, agreement_id, operation, tx_hash, status, first_seen, last_checked, confirmed_at, ledger, error_code, error_detail
		FROM transaction_refs
		WHERE status IN ('pending', 'submitted')
		ORDER BY first_seen
	`)
	if err != nil {
		return nil, fmt.Errorf("listing pending transactions: %w", err)
	}
	defer rows.Close()

	var result []TransactionRef
	for rows.Next() {
		var t TransactionRef
		if err := rows.Scan(
			&t.ID, &t.AgreementID, &t.Operation, &t.TxHash, &t.Status,
			&t.FirstSeen, &t.LastChecked, &t.ConfirmedAt, &t.Ledger, &t.ErrorCode, &t.ErrorDetail,
		); err != nil {
			return nil, fmt.Errorf("scanning transaction ref row: %w", err)
		}
		result = append(result, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating pending transactions: %w", err)
	}
	return result, nil
}
