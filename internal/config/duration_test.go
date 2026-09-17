package config

import (
	"math"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestIntervalsRejectOverflow(t *testing.T) {
	limit := math.MaxInt64 / int64(time.Second)
	for _, tc := range []struct {
		flag, env string
		server    bool
	}{
		{"-p", "POLL_INTERVAL", false},
		{"-r", "REPORT_INTERVAL", false},
		{"-i", "STORE_INTERVAL", true},
	} {
		for _, source := range []string{"flag", "env"} {
			t.Run(tc.env+"/"+source, func(t *testing.T) {
				value := strconv.FormatInt(limit+1, 10)
				args, env := []string{tc.flag + "=" + value}, map[string]string{}
				if source == "env" {
					args = nil
					env[tc.env] = value
				}
				prepareConfig(t, args, env)
				var err error
				if tc.server {
					_, err = LoadServerConfig()
				} else {
					_, err = LoadAgentConfig()
				}
				if err == nil || !strings.Contains(err.Error(), "maximum supported duration") {
					t.Errorf("overflow not rejected: %v", err)
				}
			})
		}
	}
}
