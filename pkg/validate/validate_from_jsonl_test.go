package validate

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Gunvolt24/wb_l0/internal/domain"
)

func TestValidateJSONLStream_Mixed(t *testing.T) {
	ctx := context.Background()
	validator := NewOrderValidator()

	line1 := oneLineJSONL(minimalValidOrderJSON("uid-1", "txn-1", "user1@example.com"))
	line2 := oneLineJSONL(minimalValidOrderJSON("uid-2", "txn-2", "")) // invalid email
	line3 := ""                                                        // пустая строка — ок
	line4 := oneLineJSONL(minimalValidOrderJSON("uid-3", "txn-3", "user3@example.com"))

	input := strings.Join([]string{line1, line2, line3, line4}, "\n")
	var out bytes.Buffer

	res, err := ValidateJSONLStream(ctx, validator, strings.NewReader(input), &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ValidLinesCount != 2 || res.InvalidLinesCount != 1 {
		t.Fatalf("unexpected counters: %+v", res)
	}

	outLines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(outLines) != 2 {
		t.Fatalf("expected 2 output lines, got %d", len(outLines))
	}
	var o1, o2 domain.Order
	if err := json.Unmarshal([]byte(outLines[0]), &o1); err != nil {
		t.Fatalf("unmarshal line1: %v", err)
	}
	if err := json.Unmarshal([]byte(outLines[1]), &o2); err != nil {
		t.Fatalf("unmarshal line2: %v", err)
	}
	got := []string{o1.OrderUID, o2.OrderUID}
	wantSet := map[string]bool{"uid-1": true, "uid-3": true}
	for _, uid := range got {
		if !wantSet[uid] {
			t.Fatalf("unexpected uid in output: %s", uid)
		}
	}
}

func TestValidateJSONLStream_LargeLine(t *testing.T) {
	ctx := context.Background()
	validator := NewOrderValidator()

	bigName := strings.Repeat("X", 200_000) // > 64KB
	raw := `{
	  "order_uid":"uid-big","track_number":"TN","entry":"WBIL",
	  "delivery":{"name":"N","phone":"+1","zip":"1","city":"C","address":"A","region":"R","email":"u@e.com"},
	  "payment":{"transaction":"txn-big","request_id":"","currency":"USD","provider":"wbpay","amount":1,"payment_dt":1,"bank":"b","delivery_cost":0,"goods_total":1,"custom_fee":0},
	  "items":[{"chrt_id":1,"track_number":"TN","price":1,"rid":"r","name":"` + bigName + `","sale":0,"size":"0","total_price":1,"nm_id":1,"brand":"b","status":200}],
	  "locale":"en","customer_id":"c","date_created":"2021-11-26T06:22:19Z"
	}`

	var out bytes.Buffer
	rawCompact := oneLineJSONL(raw)
	res, err := ValidateJSONLStream(ctx, validator, strings.NewReader(rawCompact+"\n"), &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ValidLinesCount != 1 || res.InvalidLinesCount != 0 {
		t.Fatalf("unexpected counters: %+v", res)
	}
	if strings.Count(strings.TrimSpace(out.String()), "\n")+1 != 1 {
		t.Fatalf("expected 1 output line")
	}
}

// ------ функции-помощники ------

func oneLineJSONL(s string) string {
	var b bytes.Buffer
	_ = json.Compact(&b, []byte(s))
	return b.String()
}
