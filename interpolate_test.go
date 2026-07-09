package tds

import (
	"database/sql/driver"
	"testing"
	"time"
)

func TestInterpolate(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		args    []driver.Value
		want    string
		wantErr bool
	}{
		{
			name:  "no_placeholders",
			query: "SELECT 1",
			args:  nil,
			want:  "SELECT 1",
		},
		{
			name:  "string_is_quoted_and_escaped",
			query: "SELECT * FROM u WHERE name = ?",
			args:  []driver.Value{"O'Brien"},
			want:  "SELECT * FROM u WHERE name = 'O''Brien'",
		},
		{
			name:  "numeric_and_bool_and_null",
			query: "INSERT INTO t VALUES (?, ?, ?, ?)",
			args:  []driver.Value{int64(42), 3.5, true, nil},
			want:  "INSERT INTO t VALUES (42, 3.5, 1, NULL)",
		},
		{
			name:  "placeholder_inside_string_literal_is_ignored",
			query: "SELECT '?' , ?",
			args:  []driver.Value{int64(7)},
			want:  "SELECT '?' , 7",
		},
		{
			name:  "placeholder_inside_line_comment_is_ignored",
			query: "SELECT ? -- what about ?\n, ?",
			args:  []driver.Value{int64(1), int64(2)},
			want:  "SELECT 1 -- what about ?\n, 2",
		},
		{
			name:  "placeholder_inside_block_comment_is_ignored",
			query: "SELECT ? /* ? ? */ , ?",
			args:  []driver.Value{int64(1), int64(2)},
			want:  "SELECT 1 /* ? ? */ , 2",
		},
		{
			name:  "doubled_quote_stays_in_string",
			query: "SELECT 'a''?b' , ?",
			args:  []driver.Value{"x"},
			want:  "SELECT 'a''?b' , 'x'",
		},
		{
			name:  "bytes_become_hex_literal",
			query: "SELECT ?",
			args:  []driver.Value{[]byte{0x00, 0xAB, 0xff}},
			want:  "SELECT 0x00abff",
		},
		{
			name:  "placeholder_inside_bracket_identifier_is_ignored",
			query: "SELECT [weird?col] FROM t WHERE x = ?",
			args:  []driver.Value{"v"},
			want:  "SELECT [weird?col] FROM t WHERE x = 'v'",
		},
		{
			name:  "doubled_bracket_stays_in_identifier",
			query: "SELECT [a]]?b] , ?",
			args:  []driver.Value{"x"},
			want:  "SELECT [a]]?b] , 'x'",
		},
		{
			name:    "too_few_args_errors",
			query:   "SELECT ?, ?",
			args:    []driver.Value{int64(1)},
			wantErr: true,
		},
		{
			name:    "too_many_args_errors",
			query:   "SELECT ?",
			args:    []driver.Value{int64(1), int64(2)},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := interpolate(tt.query, tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("interpolate(%q) =\n  %q\nwant\n  %q", tt.query, got, tt.want)
			}
		})
	}
}

func TestInterpolateTimestamp(t *testing.T) {
	ts := time.Date(2026, 7, 9, 13, 45, 30, 0, time.UTC)
	got, err := interpolate("SELECT ?", []driver.Value{ts})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "SELECT '2026-07-09 13:45:30'"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
