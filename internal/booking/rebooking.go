package booking

import (
	"fmt"
	"time"

	"github.com/VanceMichael/go-base-airbridge/internal/domain"
)

type Rebooking struct {
	ledger      *Ledger
	original    Allocation
	replacement Allocation
	openedAt    time.Time
	closed      bool
}

func (l *Ledger) BeginRebooking(allocationID string, at time.Time) (*Rebooking, error) {
	if allocationID == "" || at.IsZero() {
		return nil, domain.ErrInvalid
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	original, ok := l.allocations[allocationID]
	if !ok {
		return nil, domain.ErrNotFound
	}
	if !original.Accepted {
		return nil, domain.ErrState
	}
	l.reserved[original.Request.LegID] -= original.Request.WeightKg
	original.Accepted = false
	original.Reason = "rebooking"
	l.allocations[allocationID] = original
	return &Rebooking{ledger: l, original: original, openedAt: at}, nil
}

func (r *Rebooking) ReserveReplacement(request Request, at time.Time) (Allocation, error) {
	if r == nil || r.ledger == nil || r.closed || !r.replacement.CreatedAt.IsZero() {
		return Allocation{}, domain.ErrState
	}
	if request.TenantID != r.original.Request.TenantID || request.ShipmentID != r.original.Request.ShipmentID {
		return Allocation{}, domain.ErrForbidden
	}
	if request.LegID == "" || request.WeightKg <= 0 || at.Before(r.openedAt) {
		return Allocation{}, domain.ErrInvalid
	}

	l := r.ledger
	l.mu.Lock()
	defer l.mu.Unlock()
	capacity, ok := l.capacity[request.LegID]
	if !ok {
		return Allocation{}, domain.ErrNotFound
	}
	if l.reserved[request.LegID]+request.WeightKg > capacity {
		return Allocation{}, domain.ErrCapacity
	}
	id := fmt.Sprintf("%s-rebook-%d", request.ShipmentID, at.UnixNano())
	replacement := Allocation{ID: id, Request: request, Accepted: true, Reason: "replacement", CreatedAt: at}
	l.reserved[request.LegID] += request.WeightKg
	l.allocations[id] = replacement
	r.replacement = replacement
	return replacement, nil
}

func (r *Rebooking) Complete() error {
	if r == nil || r.ledger == nil || r.closed || r.replacement.ID == "" {
		return domain.ErrState
	}
	r.ledger.mu.Lock()
	defer r.ledger.mu.Unlock()
	original := r.ledger.allocations[r.original.ID]
	original.Reason = "rebooked"
	r.ledger.allocations[r.original.ID] = original
	r.closed = true
	return nil
}

// Abort reverses an in-flight rebooking after a downstream confirmation
// failure. The replacement reservation is released and the original
// allocation is restored, returning the ledger to its pre-change state so
// the capacity tally matches the surviving allocations.
func (r *Rebooking) Abort() error {
	if r == nil || r.ledger == nil || r.closed {
		return domain.ErrState
	}
	r.ledger.mu.Lock()
	defer r.ledger.mu.Unlock()
	if r.replacement.ID != "" {
		rep := r.ledger.allocations[r.replacement.ID]
		if rep.Accepted {
			r.ledger.reserved[rep.Request.LegID] -= rep.Request.WeightKg
			rep.Accepted = false
			rep.Reason = "aborted"
			r.ledger.allocations[r.replacement.ID] = rep
		}
	}
	orig := r.ledger.allocations[r.original.ID]
	if !orig.Accepted {
		r.ledger.reserved[orig.Request.LegID] += orig.Request.WeightKg
		orig.Accepted = true
		orig.Reason = ""
		r.ledger.allocations[r.original.ID] = orig
	}
	r.closed = true
	return nil
}

func (r *Rebooking) Original() Allocation {
	if r == nil {
		return Allocation{}
	}
	return r.original
}

func (r *Rebooking) Replacement() Allocation {
	if r == nil {
		return Allocation{}
	}
	return r.replacement
}
