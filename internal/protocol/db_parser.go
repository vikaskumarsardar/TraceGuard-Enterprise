package protocol

import (
	"regexp"
	"strings"
	"time"

	"traceguard/internal/domain"

	"github.com/google/uuid"
)

var sqlCommentRegex = regexp.MustCompile(`/\*\s*(.*?)\s*\*/`)

// DBParser parses database wire protocol packets and extracts query details & SQL comments.
type DBParser struct{}

func NewDBParser() *DBParser {
	return &DBParser{}
}

// ParsePostgresQuery parses raw bytes from PostgreSQL client/server protocol.
func (p *DBParser) ParsePostgresQuery(payload []byte, clientIP, serverIP string) *domain.Span {
	strPayload := string(payload)

	// Check if this is a query statement or simple SQL text
	if !strings.Contains(strPayload, "SELECT") &&
		!strings.Contains(strPayload, "INSERT") &&
		!strings.Contains(strPayload, "UPDATE") &&
		!strings.Contains(strPayload, "DELETE") {
		return nil
	}

	sqlQuery := sanitizeSQL(strPayload)
	correlationID, traceID := extractSQLCommentMeta(sqlQuery)

	if traceID == "" {
		traceID = uuid.New().String()
	}
	if correlationID == "" {
		correlationID = traceID
	}

	return &domain.Span{
		ID:            uuid.New().String(),
		TraceID:       traceID,
		CorrelationID: correlationID,
		ServiceName:   "database-postgres",
		Protocol:      domain.ProtocolPostgres,
		Name:          extractQueryAction(sqlQuery),
		SQLQuery:      sqlQuery,
		StartTime:     time.Now(),
		Status:        domain.StatusOk,
		ClientIP:      clientIP,
		ServerIP:      serverIP,
		Attributes: map[string]string{
			"db.system": "postgresql",
			"db.statement": sqlQuery,
		},
	}
}

// ParseRedisCommand decodes Redis RESP protocol payloads.
func (p *DBParser) ParseRedisCommand(payload []byte, clientIP, serverIP string) *domain.Span {
	strPayload := string(payload)
	if !strings.HasPrefix(strPayload, "*") && !strings.HasPrefix(strPayload, "$") {
		return nil
	}

	command := parseRESPCommand(strPayload)
	if command == "" {
		command = "REDIS COMMAND"
	}

	traceID := uuid.New().String()
	return &domain.Span{
		ID:            traceID,
		TraceID:       traceID,
		CorrelationID: traceID,
		ServiceName:   "cache-redis",
		Protocol:      domain.ProtocolRedis,
		Name:          "REDIS " + command,
		StartTime:     time.Now(),
		Status:        domain.StatusOk,
		ClientIP:      clientIP,
		ServerIP:      serverIP,
		Attributes: map[string]string{
			"db.system": "redis",
			"redis.command": command,
		},
	}
}

func extractSQLCommentMeta(sql string) (correlationID, traceID string) {
	matches := sqlCommentRegex.FindStringSubmatch(sql)
	if len(matches) > 1 {
		comment := matches[1]
		parts := strings.Split(comment, ",")
		for _, part := range parts {
			kv := strings.Split(strings.TrimSpace(part), "=")
			if len(kv) == 2 {
				key := strings.Trim(kv[0], "'\" ")
				val := strings.Trim(kv[1], "'\" ")
				if key == "x-correlation-id" || key == "correlation_id" {
					correlationID = val
				}
				if key == "traceparent" || key == "trace_id" {
					traceID = val
				}
			}
		}
	}
	return correlationID, traceID
}

func sanitizeSQL(sql string) string {
	sql = strings.ReplaceAll(sql, "\x00", "")
	idx := strings.Index(strings.ToUpper(sql), "SELECT")
	if idx == -1 {
		idx = strings.Index(strings.ToUpper(sql), "INSERT")
	}
	if idx == -1 {
		idx = strings.Index(strings.ToUpper(sql), "UPDATE")
	}
	if idx == -1 {
		idx = strings.Index(strings.ToUpper(sql), "DELETE")
	}
	if idx != -1 {
		return sql[idx:]
	}
	return sql
}

func extractQueryAction(sql string) string {
	upper := strings.ToUpper(sql)
	if strings.HasPrefix(upper, "SELECT") {
		return "SQL SELECT"
	}
	if strings.HasPrefix(upper, "INSERT") {
		return "SQL INSERT"
	}
	if strings.HasPrefix(upper, "UPDATE") {
		return "SQL UPDATE"
	}
	if strings.HasPrefix(upper, "DELETE") {
		return "SQL DELETE"
	}
	return "SQL QUERY"
}

func parseRESPCommand(resp string) string {
	lines := strings.Split(resp, "\r\n")
	for _, l := range lines {
		if len(l) > 0 && !strings.HasPrefix(l, "$") && !strings.HasPrefix(l, "*") {
			return strings.ToUpper(l)
		}
	}
	return "COMMAND"
}
