package booking

import (
	"context"
	"errors"
	"testing"
	"time"
)

type changeRecorderFunc func(context.Context, ChangeRecord) error

func (f changeRecorderFunc) RecordChange(ctx context.Context, record ChangeRecord) error {
	return f(ctx, record)
}

func TestFailedSameLegRebookingRestoresOriginalAllocation(t *testing.T) {
	now := time.Date(2026, 8, 21, 4, 0, 0, 0, time.UTC)
	ledger := NewLedger()
	if err := ledger.DefineLeg("PVG-FRA", 100); err != nil {
		t.Fatalf("define leg: %v", err)
	}
	original, err := ledger.Reserve(context.Background(), Request{
		TenantID: "cargo-east", ShipmentID: "shipment-13", LegID: "PVG-FRA", WeightKg: 60, RequestedAt: now,
	})
	if err != nil {
		t.Fatalf("reserve original: %v", err)
	}

	recorderErr := errors.New("audit storage unavailable")
	service := ChangeService{
		Ledger:   ledger,
		Recorder: changeRecorderFunc(func(context.Context, ChangeRecord) error { return recorderErr }),
		Now:      func() time.Time { return now.Add(time.Minute) },
	}
	_, err = service.Change(context.Background(), ChangeRequest{
		AllocationID: original.ID,
		Replacement:  Request{TenantID: "cargo-east", ShipmentID: "shipment-13", LegID: "PVG-FRA", WeightKg: 80, RequestedAt: now.Add(time.Minute)},
		RequestedBy:  "dispatcher-13",
		RequestedAt:  now.Add(time.Minute),
	})
	if !errors.Is(err, recorderErr) {
		t.Fatalf("change error = %v, want recorder failure", err)
	}

	allocations := ledger.List("cargo-east")
	if len(allocations) != 2 {
		t.Fatalf("allocations after failed rebooking = %d, want original only plus rejected history", len(allocations))
	}
	var restored, replacementAfterFailure Allocation
	for _, allocation := range allocations {
		if allocation.ID == original.ID {
			restored = allocation
		} else {
			replacementAfterFailure = allocation
		}
	}
	if !restored.Accepted || restored.ID != original.ID || restored.Reason != "" {
		t.Fatalf("original allocation after failure = %+v, want restored", restored)
	}
	if replacementAfterFailure.Accepted {
		t.Fatalf("replacement remained active after failure: %+v", replacementAfterFailure)
	}
	available, err := ledger.Available("PVG-FRA")
	if err != nil {
		t.Fatalf("available: %v", err)
	}
	if available != 40 {
		t.Fatalf("available capacity after rollback = %d, want 40", available)
	}

	service.Recorder = changeRecorderFunc(func(context.Context, ChangeRecord) error { return nil })
	replacement, err := service.Change(context.Background(), ChangeRequest{
		AllocationID: original.ID,
		Replacement:  Request{TenantID: "cargo-east", ShipmentID: "shipment-13", LegID: "PVG-FRA", WeightKg: 80, RequestedAt: now.Add(2 * time.Minute)},
		RequestedBy:  "dispatcher-13",
		RequestedAt:  now.Add(2 * time.Minute),
	})
	if err != nil {
		t.Fatalf("successful rebooking: %v", err)
	}
	if !replacement.Accepted || replacement.Request.WeightKg != 80 {
		t.Fatalf("replacement = %+v", replacement)
	}
	available, _ = ledger.Available("PVG-FRA")
	if available != 20 {
		t.Fatalf("available after successful rebooking = %d, want 20", available)
	}
}
