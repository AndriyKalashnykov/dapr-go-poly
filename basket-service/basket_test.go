package main

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

// Most basket-service routes are currently commented out (WIP); these tests
// pin down the data-type contracts so future route activation doesn't silently
// regress the wire shape consumed by other services.

func TestBasketItem_JSONShape_UsesCatalogIdFieldName(t *testing.T) {
	t.Parallel()

	productID := uuid.MustParse("72119506-89ef-4c0c-ace7-6cbd984bfc50")
	item := BasketItem{
		Id:        uuid.New(),
		ProductId: productID,
		Price:     9.99,
		Quantity:  2,
	}

	raw, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("marshal basket item: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal to map: %v", err)
	}

	if _, ok := decoded["catalogId"]; !ok {
		t.Errorf("expected JSON field 'catalogId' (ProductId alias), got keys: %v", keys(decoded))
	}
	if _, ok := decoded["quantity"]; !ok {
		t.Errorf("expected JSON field 'quantity', got keys: %v", keys(decoded))
	}
}

func TestCreateBasketRequest_RoundTripsThroughJSON(t *testing.T) {
	t.Parallel()

	original := CreateBasketRequest{
		Items: []CreateBasketItemRequest{
			{ProductId: "72119506-89ef-4c0c-ace7-6cbd984bfc50", Quantity: 3},
			{ProductId: "7f1b3f7a-0f9b-4f2a-9fa1-1aeb2f4c2f00", Quantity: 1},
		},
	}

	raw, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var round CreateBasketRequest
	if err := json.Unmarshal(raw, &round); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got, want := len(round.Items), 2; got != want {
		t.Fatalf("items length = %d, want %d", got, want)
	}
	if round.Items[0].ProductId != original.Items[0].ProductId {
		t.Errorf("productId[0] = %q, want %q", round.Items[0].ProductId, original.Items[0].ProductId)
	}
	if round.Items[1].Quantity != original.Items[1].Quantity {
		t.Errorf("quantity[1] = %d, want %d", round.Items[1].Quantity, original.Items[1].Quantity)
	}
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
