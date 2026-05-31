package repository

import (
	"errors"
	"testing"

	"github.com/jackc/pgerrcode"
	"github.com/lib/pq"
)

func TestIsPostgresConnectionException(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "connection exception class 08",
			err:  &pq.Error{Code: pq.ErrorCode(pgerrcode.ConnectionFailure)},
			want: true,
		},
		{
			name: "non-retriable postgres error",
			err:  &pq.Error{Code: pq.ErrorCode(pgerrcode.UniqueViolation)},
			want: false,
		},
		{
			name: "non-postgres error",
			err:  errors.New("plain error"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPostgresConnectionException(tt.err); got != tt.want {
				t.Fatalf("unexpected result: got %t want %t", got, tt.want)
			}
		})
	}
}
