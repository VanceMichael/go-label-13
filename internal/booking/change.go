package booking

import (
	"context"
	"fmt"
	"time"
)

type ChangeRequest struct {
	AllocationID string
	Replacement  Request
	RequestedBy  string
	RequestedAt  time.Time
}

type ChangeRecord struct {
	OriginalID    string
	ReplacementID string
	RequestedBy   string
	ChangedAt     time.Time
}

type ChangeRecorder interface {
	RecordChange(context.Context, ChangeRecord) error
}

type ChangeService struct {
	Ledger   *Ledger
	Recorder ChangeRecorder
	Now      func() time.Time
}

func (s ChangeService) Change(ctx context.Context, request ChangeRequest) (Allocation, error) {
	if s.Ledger == nil || s.Recorder == nil {
		return Allocation{}, fmt.Errorf("booking change dependencies are required")
	}
	if request.AllocationID == "" || request.RequestedBy == "" || request.RequestedAt.IsZero() {
		return Allocation{}, fmt.Errorf("booking change identity is required")
	}
	if err := ctx.Err(); err != nil {
		return Allocation{}, err
	}

	rebooking, err := s.Ledger.BeginRebooking(request.AllocationID, request.RequestedAt)
	if err != nil {
		return Allocation{}, err
	}
	replacement, err := rebooking.ReserveReplacement(request.Replacement, s.now())
	if err != nil {
		return Allocation{}, fmt.Errorf("reserve replacement: %w", err)
	}
	if err := s.Recorder.RecordChange(ctx, ChangeRecord{
		OriginalID:    rebooking.Original().ID,
		ReplacementID: replacement.ID,
		RequestedBy:   request.RequestedBy,
		ChangedAt:     s.now(),
	}); err != nil {
		return Allocation{}, fmt.Errorf("record booking change: %w", err)
	}
	if err := rebooking.Complete(); err != nil {
		return Allocation{}, fmt.Errorf("complete booking change: %w", err)
	}
	return replacement, nil
}

func (s ChangeService) now() time.Time {
	if s.Now != nil {
		if now := s.Now(); !now.IsZero() {
			return now.UTC()
		}
	}
	return time.Now().UTC()
}
