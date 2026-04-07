package agro

import (
	"context"
	"testing"
	"time"

	domain "github.com/rfulgencio3/go-whatsapp-ai-integration-POC/internal/domain/agro"
)

func TestRuleBasedInterpreterInterpret(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.April, 6, 10, 30, 0, 0, time.UTC)
	interpreter := NewRuleBasedInterpreter()

	testCases := []struct {
		name             string
		text             string
		expectedIntent   string
		expectedCategory string
		expectedSubcat   string
		expectedAmount   *float64
		expectedQuantity *float64
		expectedUnit     string
		expectOccurredAt bool
	}{
		{
			name:             "input purchase",
			text:             "Comprei 10 sacos de ração por 850 reais",
			expectedIntent:   "finance.input_purchase",
			expectedCategory: "finance",
			expectedSubcat:   "input_purchase",
			expectedAmount:   float64Ptr(850),
			expectedQuantity: float64Ptr(10),
			expectedUnit:     "saco",
		},
		{
			name:             "generic expense",
			text:             "Paguei 300 reais de veterinário",
			expectedIntent:   "finance.expense",
			expectedCategory: "finance",
			expectedSubcat:   "expense",
			expectedAmount:   float64Ptr(300),
		},
		{
			name:             "revenue",
			text:             "Recebi 1200 reais pela venda de leite",
			expectedIntent:   "finance.revenue",
			expectedCategory: "finance",
			expectedSubcat:   "revenue",
			expectedAmount:   float64Ptr(1200),
		},
		{
			name:             "insemination",
			text:             "A vaca 32 foi inseminada hoje",
			expectedIntent:   "reproduction.insemination",
			expectedCategory: "reproduction",
			expectedSubcat:   "insemination",
			expectOccurredAt: true,
		},
		{
			name:             "mastitis treatment",
			text:             "A vaca 32 esta com problema nas tetas T1 e T3 e nao pode tirar leite",
			expectedIntent:   "health.mastitis_treatment",
			expectedCategory: "health",
			expectedSubcat:   "mastitis_treatment",
		},
		{
			name:             "hoof treatment",
			text:             "A vaca 18 esta mancando por problema de casco",
			expectedIntent:   "health.hoof_treatment",
			expectedCategory: "health",
			expectedSubcat:   "hoof_treatment",
		},
		{
			name:             "bloat",
			text:             "A vaca 21 esta com gases e barriga inchada",
			expectedIntent:   "health.bloat",
			expectedCategory: "health",
			expectedSubcat:   "bloat",
		},
		{
			name:             "fallback note",
			text:             "Choveu forte no talhao 4",
			expectedIntent:   "operations.note",
			expectedCategory: "operations",
			expectedSubcat:   "note",
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result, err := interpreter.Interpret(context.Background(), InterpretationInput{
				MessageType: domain.MessageTypeText,
				Text:        testCase.text,
				OccurredAt:  now,
			})
			if err != nil {
				t.Fatalf("Interpret() error = %v", err)
			}
			if result.NormalizedIntent != testCase.expectedIntent {
				t.Fatalf("expected intent %q, got %q", testCase.expectedIntent, result.NormalizedIntent)
			}
			if result.Category != testCase.expectedCategory {
				t.Fatalf("expected category %q, got %q", testCase.expectedCategory, result.Category)
			}
			if result.Subcategory != testCase.expectedSubcat {
				t.Fatalf("expected subcategory %q, got %q", testCase.expectedSubcat, result.Subcategory)
			}
			assertFloatPtr(t, "amount", testCase.expectedAmount, result.Amount)
			assertFloatPtr(t, "quantity", testCase.expectedQuantity, result.Quantity)
			if result.Unit != testCase.expectedUnit {
				t.Fatalf("expected unit %q, got %q", testCase.expectedUnit, result.Unit)
			}
			if testCase.expectOccurredAt && result.OccurredAt == nil {
				t.Fatalf("expected occurred_at to be resolved")
			}
			if !testCase.expectOccurredAt && result.OccurredAt != nil {
				t.Fatalf("did not expect occurred_at, got %v", result.OccurredAt)
			}
			if result.RawOutputJSON == "" {
				t.Fatalf("expected raw output json")
			}
			if testCase.expectedCategory == "health" && result.AnimalCode == "" {
				t.Fatalf("expected animal code for health event")
			}
		})
	}
}

func float64Ptr(value float64) *float64 {
	return &value
}

func assertFloatPtr(t *testing.T, label string, expected, actual *float64) {
	t.Helper()

	switch {
	case expected == nil && actual == nil:
		return
	case expected == nil && actual != nil:
		t.Fatalf("expected %s nil, got %v", label, *actual)
	case expected != nil && actual == nil:
		t.Fatalf("expected %s %v, got nil", label, *expected)
	case *expected != *actual:
		t.Fatalf("expected %s %v, got %v", label, *expected, *actual)
	}
}
