package transport_test

import (
	"testing"

	"github.com/johnkil/outlook-agent/internal/transport"
)

func TestClampPageSizeDefaultsMissingAndNonPositiveValues(t *testing.T) {
	for name, value := range map[string]any{
		"missing":  nil,
		"zero":     0,
		"negative": -1,
	} {
		t.Run(name, func(t *testing.T) {
			result, err := transport.ClampPageSize(value, 150, 250)
			if err != nil {
				t.Fatalf("clamp page size: %v", err)
			}
			if result.Value != 150 || result.Clamped {
				t.Fatalf("expected default without clamp marker, got %#v", result)
			}
		})
	}
}

func TestClampPageSizeClampsHugeValuesAcrossNumericForms(t *testing.T) {
	for name, value := range map[string]any{
		"int":     1_000_000,
		"float":   float64(1_000_000),
		"string":  "1000000",
		"int64":   int64(1_000_000),
		"uint64":  uint64(1_000_000),
		"float32": float32(1_000_000),
	} {
		t.Run(name, func(t *testing.T) {
			result, err := transport.ClampPageSize(value, 150, 250)
			if err != nil {
				t.Fatalf("clamp page size: %v", err)
			}
			if result.Value != 250 || !result.Clamped {
				t.Fatalf("expected max clamp, got %#v", result)
			}
		})
	}
}

func TestClampPageSizeRejectsInvalidValues(t *testing.T) {
	for name, value := range map[string]any{
		"fractional_float": 10.5,
		"fractional_text":  "10.5",
		"bool":             true,
		"object":           map[string]any{"max": 10},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := transport.ClampPageSize(value, 150, 250); err == nil {
				t.Fatal("expected invalid page size to fail")
			}
		})
	}
}

func TestClampPageSizeRejectsInvalidBounds(t *testing.T) {
	if _, err := transport.ClampPageSize(10, 0, 250); err == nil {
		t.Fatal("expected invalid default size to fail")
	}
	if _, err := transport.ClampPageSize(10, 150, 100); err == nil {
		t.Fatal("expected max below default to fail")
	}
}
