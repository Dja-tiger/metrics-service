package agent

import (
	"testing"
	"time"

	models "github.com/Dja-tiger/metrics-service/internal/model"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type batchSenderFunc func([]models.Metrics) error

func (f batchSenderFunc) Send(m []models.Metrics) error { return f(m) }

func TestGRPCSenderRetries(t *testing.T) {
	for _, code := range []codes.Code{codes.Unavailable, codes.PermissionDenied, codes.InvalidArgument} {
		t.Run(code.String(), func(t *testing.T) {
			calls := 0
			var sleeps []time.Duration
			sender := batchSenderFunc(func(m []models.Metrics) error {
				calls++
				if len(m) != 1 {
					t.Fatal("batch changed")
				}
				return status.Error(code, "test")
			})
			a, err := NewAgent("http://localhost:8080", time.Second, time.Second, WithBatchSender(sender), WithRetrySleep(func(d time.Duration) { sleeps = append(sleeps, d) }))
			if err != nil {
				t.Fatal(err)
			}
			if err = a.sendMetrics(nil); err != nil || calls != 0 {
				t.Fatal("empty batch sent")
			}
			if err = a.sendMetrics([]models.Metrics{{ID: "test"}}); status.Code(err) != code {
				t.Fatal(err)
			}
			want := 1
			if code == codes.Unavailable {
				want = 4
				if len(sleeps) != 3 || sleeps[0] != time.Second || sleeps[1] != 3*time.Second || sleeps[2] != 5*time.Second {
					t.Fatalf("delays %v", sleeps)
				}
			}
			if calls != want {
				t.Fatalf("calls %d want %d", calls, want)
			}
		})
	}
}
