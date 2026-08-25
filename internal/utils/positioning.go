package utils

import (
	"context"
	"net/http"
	"slices"

	"github.com/google/uuid"
	"github.com/nazifbara/kanban-api/internal/apierrors"
)

func isPositionInRange(position int32, size int32, onInsert bool) bool {
	if position < 0 {
		return false
	}
	if onInsert {
		return position <= size
	}
	return position < size
}

type PositionParam[T any] struct {
	Items      []T
	ItemID     func(T) uuid.UUID
	ItemPos    func(T) int32
	UpdateItem func(ctx context.Context, id uuid.UUID, pos int32) error
	TargetID   uuid.UUID
	Position   int32
}

func PositionItem[T any](ctx context.Context, p PositionParam[T]) error {
	oldPosition := IntToInt32(slices.IndexFunc(p.Items, func(i T) bool {
		return p.ItemID(i) == p.TargetID
	}))

	size := IntToInt32(len(p.Items))

	if !isPositionInRange(p.Position, size, oldPosition == -1) {
		return apierrors.New(http.StatusBadRequest, "position out of range")
	}

	stopIdx := size
	if oldPosition != -1 {
		stopIdx = oldPosition
	}

	shift := func(idx int32, delta int32) error {
		item := p.Items[idx]
		if err := p.UpdateItem(ctx, p.ItemID(item), p.ItemPos(item)+delta); err != nil {
			return apierrors.FromDBErr(err)
		}
		return nil
	}

	switch {
	case oldPosition == -1:
		for i := p.Position; i < size; i++ {
			if err := shift(i, 1); err != nil {
				return err
			}
		}
	case oldPosition > p.Position:
		for i := p.Position; i < stopIdx; i++ {
			if err := shift(i, 1); err != nil {
				return err
			}
		}
	case oldPosition < p.Position:
		for i := p.Position; i > stopIdx; i-- {
			if err := shift(i, -1); err != nil {
				return err
			}
		}
	}

	return nil
}
