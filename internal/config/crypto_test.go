package config

import (
	"flag"
	"os"
	"testing"
)

func TestCryptoKeyConfiguration(t *testing.T) {
	for _, app := range []string{"agent", "server"} {
		t.Run(app, func(t *testing.T) {
			for _, tc := range []struct {
				name   string
				args   []string
				env    string
				setEnv bool
				want   string
			}{
				{name: "default"},
				{name: "flag", args: []string{"-crypto-key=flag.pem"}, want: "flag.pem"},
				{name: "environment", env: "env.pem", setEnv: true, want: "env.pem"},
				{name: "environment wins", args: []string{"-crypto-key=flag.pem"}, env: "env.pem", setEnv: true, want: "env.pem"},
				{name: "empty environment disables", args: []string{"-crypto-key=flag.pem"}, setEnv: true},
			} {
				t.Run(tc.name, func(t *testing.T) {
					for _, name := range []string{"TRUSTED_SUBNET", "CONFIG", "STORE_FILE", "ADDRESS", "REPORT_INTERVAL", "POLL_INTERVAL", "RATE_LIMIT", "KEY", "STORE_INTERVAL", "FILE_STORAGE_PATH", "RESTORE", "DATABASE_DSN", "AUDIT_FILE", "AUDIT_URL", "CRYPTO_KEY"} {
						t.Setenv(name, "")
						if err := os.Unsetenv(name); err != nil {
							t.Fatal(err)
						}
					}
					if tc.setEnv {
						t.Setenv("CRYPTO_KEY", tc.env)
					}
					oldFlags, oldArgs := flag.CommandLine, os.Args
					t.Cleanup(func() { flag.CommandLine, os.Args = oldFlags, oldArgs })
					flag.CommandLine = flag.NewFlagSet(app, flag.ContinueOnError)
					os.Args = append([]string{app}, tc.args...)
					var got string
					if app == "agent" {
						cfg, err := LoadAgentConfig()
						if err != nil {
							t.Fatal(err)
						}
						got = cfg.CryptoKey
					} else {
						cfg, err := LoadServerConfig()
						if err != nil {
							t.Fatal(err)
						}
						got = cfg.CryptoKey
					}
					if got != tc.want {
						t.Fatalf("CryptoKey=%q, want %q", got, tc.want)
					}
				})
			}
		})
	}
}
