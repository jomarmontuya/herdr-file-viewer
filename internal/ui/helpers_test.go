package ui

import "testing"

func TestClampScrollStart(t *testing.T) {
	tests := []struct {
		name               string
		start, total, rows int
		want               int
	}{
		{name: "negative", start: -4, total: 20, rows: 5, want: 0},
		{name: "middle", start: 7, total: 20, rows: 5, want: 7},
		{name: "past end", start: 30, total: 20, rows: 5, want: 15},
		{name: "content fits", start: 4, total: 5, rows: 5, want: 0},
		{name: "no rows", start: 4, total: 20, rows: 0, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clampScrollStart(tt.start, tt.total, tt.rows); got != tt.want {
				t.Fatalf("clampScrollStart(%d, %d, %d) = %d, want %d", tt.start, tt.total, tt.rows, got, tt.want)
			}
		})
	}
}

func TestRevealScrollStart(t *testing.T) {
	tests := []struct {
		name                       string
		start, cursor, total, rows int
		want                       int
	}{
		{name: "cursor already visible", start: 5, cursor: 7, total: 20, rows: 5, want: 5},
		{name: "cursor above viewport", start: 5, cursor: 2, total: 20, rows: 5, want: 2},
		{name: "cursor below viewport", start: 5, cursor: 12, total: 20, rows: 5, want: 8},
		{name: "cursor at end", start: 0, cursor: 19, total: 20, rows: 5, want: 15},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := revealScrollStart(tt.start, tt.cursor, tt.total, tt.rows); got != tt.want {
				t.Fatalf("revealScrollStart(%d, %d, %d, %d) = %d, want %d", tt.start, tt.cursor, tt.total, tt.rows, got, tt.want)
			}
		})
	}
}
