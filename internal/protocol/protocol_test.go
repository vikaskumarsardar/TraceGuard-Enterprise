package protocol

import (
	"testing"
)

func TestExtractSQLCommentMeta(t *testing.T) {
	sql := "SELECT * FROM orders WHERE id = 42 /* x-correlation-id='req-999',traceparent='00-abc-123-01' */;"
	corrID, traceID := extractSQLCommentMeta(sql)

	if corrID != "req-999" {
		t.Errorf("expected correlation ID 'req-999', got '%s'", corrID)
	}

	if traceID != "00-abc-123-01" {
		t.Errorf("expected traceparent '00-abc-123-01', got '%s'", traceID)
	}
}
