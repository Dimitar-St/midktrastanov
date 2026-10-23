package main

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAddMonthsClamped(t *testing.T) {
	for _, test := range []struct {
		anchor string
		months int
		want   string
	}{
		{"2024-01-31T09:00:00Z", 1, "2024-02-29"},
		{"2025-01-31T09:00:00Z", 1, "2025-02-28"},
		{"2024-02-29T09:00:00Z", 12, "2025-02-28"},
	} {
		anchor, _ := time.Parse(time.RFC3339, test.anchor)
		if got := addMonthsClamped(anchor, test.months).Format("2006-01-02"); got != test.want {
			t.Errorf("%s + %d months = %s, want %s", test.anchor, test.months, got, test.want)
		}
	}
}

func TestInitialMigrationDefinesSchedulerTables(t *testing.T) {
	sql, err := migrations.ReadFile("migrations/0001_initial_schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"CREATE SCHEMA IF NOT EXISTS scheduler", "scheduler.companies", "scheduler.users", "scheduler.events"} {
		if !strings.Contains(string(sql), table) {
			t.Errorf("initial migration does not define %s", table)
		}
	}
}

func TestSignedSessionRejectsModifiedAndExpiredTokens(t *testing.T) {
	server := &Server{sessionSecret: []byte("test-secret")}
	token, err := server.signSession(sessionClaims{UserID: "user", CompanyID: "company", ExpiresAt: time.Now().Add(time.Hour).Unix()})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("GET", "/", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	if claims, err := server.session(request); err != nil || claims.UserID != "user" {
		t.Fatal(claims, err)
	}
	request.Header.Set("Authorization", "Bearer "+token+"x")
	if _, err := server.session(request); err == nil {
		t.Fatal("modified token was accepted")
	}
	expired, _ := server.signSession(sessionClaims{UserID: "user", CompanyID: "company", ExpiresAt: time.Now().Add(-time.Hour).Unix()})
	request.Header.Set("Authorization", "Bearer "+expired)
	if _, err := server.session(request); err == nil {
		t.Fatal("expired token was accepted")
	}
}
