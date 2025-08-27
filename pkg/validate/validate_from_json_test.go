package validate

import (
	"context"
	"strings"
	"testing"
)

func TestValidateOrderFromJSON_OK(t *testing.T) {
	ctx := context.Background()
	validator := NewOrderValidator()

	validJSON := minimalValidOrderJSON("uid-1", "txn-1", "user@example.com")

	order, err := ValidateOrderFromJSON(ctx, validator, []byte(validJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if order.OrderUID != "uid-1" {
		t.Fatalf("unexpected order uid: %s", order.OrderUID)
	}
}

func TestValidateOrderFromJSON_UnknownField(t *testing.T) {
	ctx := context.Background()
	validator := NewOrderValidator()

	raw := `{"unknown":"x",` + minimalValidOrderJSONFields("uid-2", "txn-2", "user@example.com")[1:]
	_, err := ValidateOrderFromJSON(ctx, validator, []byte(raw))
	if err == nil || !strings.Contains(err.Error(), "invalid json") {
		t.Fatalf("expected invalid json error, got: %v", err)
	}
}

func TestValidateOrderFromJSON_TrailingData(t *testing.T) {
	ctx := context.Background()
	validator := NewOrderValidator()

	raw := minimalValidOrderJSON("uid-3", "txn-3", "user@example.com") + "{}"
	_, err := ValidateOrderFromJSON(ctx, validator, []byte(raw))
	if err == nil || !strings.Contains(err.Error(), "trailing data") {
		t.Fatalf("expected trailing data error, got: %v", err)
	}
}

func TestValidateOrderFromJSON_DomainError(t *testing.T) {
	ctx := context.Background()
	validator := NewOrderValidator()

	// Не валиден: пустой email
	raw := minimalValidOrderJSON("uid-4", "txn-4", "")
	_, err := ValidateOrderFromJSON(ctx, validator, []byte(raw))
	if err == nil {
		t.Fatalf("expected domain validation error, got nil")
	}
}

// ---- helpers ----

func minimalValidOrderJSON(orderUID, transaction, email string) string {
	return `{
  "order_uid": "` + orderUID + `",
  "track_number": "TN",
  "entry": "WBIL",
  "delivery": {
    "name":"N","phone":"+1","zip":"1","city":"C","address":"A","region":"R","email":"` + email + `"
  },
  "payment": {
    "transaction":"` + transaction + `","request_id":"","currency":"USD","provider":"wbpay",
    "amount":1,"payment_dt":1,"bank":"b","delivery_cost":0,"goods_total":1,"custom_fee":0
  },
  "items":[{"chrt_id":1,"track_number":"TN","price":1,"rid":"r","name":"x","sale":0,"size":"0","total_price":1,"nm_id":1,"brand":"b","status":200}],
  "locale":"en","internal_signature":"","customer_id":"c","delivery_service":"d","shardkey":"9","sm_id":1,
  "date_created":"2021-11-26T06:22:19Z","oof_shard":"1"
}`
}

func minimalValidOrderJSONFields(orderUID, transaction, email string) string {
	// То же, но без ведущей '{', удобно для инъекции "unknown" в начало.
	return ` "order_uid": "` + orderUID + `",
  "track_number": "TN",
  "entry": "WBIL",
  "delivery": {"name":"N","phone":"+1","zip":"1","city":"C","address":"A","region":"R","email":"` + email + `"},
  "payment": {"transaction":"` + transaction + `","request_id":"","currency":"USD","provider":"wbpay","amount":1,"payment_dt":1,"bank":"b","delivery_cost":0,"goods_total":1,"custom_fee":0},
  "items":[{"chrt_id":1,"track_number":"TN","price":1,"rid":"r","name":"x","sale":0,"size":"0","total_price":1,"nm_id":1,"brand":"b","status":200}],
  "locale":"en","internal_signature":"","customer_id":"c","delivery_service":"d","shardkey":"9","sm_id":1,
  "date_created":"2021-11-26T06:22:19Z","oof_shard":"1"
}`
}
