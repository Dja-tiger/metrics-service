package config

import (
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func prepareConfig(t *testing.T, args []string, env map[string]string) {
	t.Helper()
	for _, name := range []string{"TRUSTED_SUBNET", "CONFIG", "STORE_FILE", "ADDRESS", "REPORT_INTERVAL", "POLL_INTERVAL", "RATE_LIMIT", "KEY", "STORE_INTERVAL", "FILE_STORAGE_PATH", "RESTORE", "DATABASE_DSN", "AUDIT_FILE", "AUDIT_URL", "CRYPTO_KEY"} {
		t.Setenv(name, "")
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
	}
	for name, value := range env {
		t.Setenv(name, value)
	}
	oldFlags, oldArgs := flag.CommandLine, os.Args
	t.Cleanup(func() { flag.CommandLine, os.Args = oldFlags, oldArgs })
	flag.CommandLine = flag.NewFlagSet("test", flag.ContinueOnError)
	os.Args = append([]string{"test"}, args...)
}

func configFile(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAgentFilePrecedence(t *testing.T) {
	path := configFile(t, `{"address":"file:8000","report_interval":"1m","poll_interval":"3s","rate_limit":5,"key":"file","crypto_key":"file.pem"}`)
	for _, tc := range []struct {
		name string
		args []string
		env  map[string]string
		want AgentConfig
	}{
		{"file", nil, nil, AgentConfig{Address: "http://file:8000", ReportInterval: 60, PollInterval: 3, RateLimit: 5, Key: "file", CryptoKey: "file.pem"}},
		{"flags including defaults and empty", []string{"-a=localhost:8080", "-r=10", "-p=2", "-l=1", "-k=", "-crypto-key="}, nil, AgentConfig{Address: "http://localhost:8080", ReportInterval: 10, PollInterval: 2, RateLimit: 1}},
		{"environment", []string{"-a=flag:8000", "-r=10", "-p=2", "-l=1", "-k=flag", "-crypto-key=flag.pem"}, map[string]string{"ADDRESS": "env:9000", "REPORT_INTERVAL": "20", "POLL_INTERVAL": "4", "RATE_LIMIT": "3", "KEY": "env", "CRYPTO_KEY": "env.pem"}, AgentConfig{Address: "http://env:9000", ReportInterval: 20, PollInterval: 4, RateLimit: 3, Key: "env", CryptoKey: "env.pem"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prepareConfig(t, append([]string{"-c", path}, tc.args...), tc.env)
			got, err := LoadAgentConfig()
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestServerFilePrecedence(t *testing.T) {
	path := configFile(t, `{"address":"file:8000","restore":false,"store_interval":"0s","store_file":"file.json","database_dsn":"file-dsn","key":"file","crypto_key":"file.pem","audit_file":"audit.log","audit_url":"http://audit"}`)
	for _, tc := range []struct {
		name string
		args []string
		env  map[string]string
		want ServerConfig
	}{
		{"file", nil, nil, ServerConfig{Address: "file:8000", StoreInterval: 0, FileStoragePath: "file.json", FileStorageEnabled: true, Restore: false, DatabaseDSN: "file-dsn", Key: "file", CryptoKey: "file.pem", AuditFile: "audit.log", AuditURL: "http://audit"}},
		{"flags including defaults and empty", []string{"-a=localhost:8080", "-i=300", "-f=", "-r=true", "-d=", "-k=", "-crypto-key=", "-audit-file=", "-audit-url="}, nil, ServerConfig{Address: "localhost:8080", StoreInterval: 300, Restore: true}},
		{"environment", []string{"-a=flag:8000", "-i=300", "-f=flag.json", "-r=true", "-d=flag", "-k=flag", "-crypto-key=flag", "-audit-file=flag", "-audit-url=flag"}, map[string]string{"ADDRESS": "env:9000", "STORE_INTERVAL": "5", "FILE_STORAGE_PATH": "env.json", "RESTORE": "false", "DATABASE_DSN": "env-dsn", "KEY": "env", "CRYPTO_KEY": "env.pem", "AUDIT_FILE": "env.log", "AUDIT_URL": "http://env"}, ServerConfig{Address: "env:9000", StoreInterval: 5, FileStoragePath: "env.json", FileStorageEnabled: true, Restore: false, DatabaseDSN: "env-dsn", Key: "env", CryptoKey: "env.pem", AuditFile: "env.log", AuditURL: "http://env"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prepareConfig(t, append([]string{"-config", path}, tc.args...), tc.env)
			got, err := LoadServerConfig()
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestFileSelectionAndDefaults(t *testing.T) {
	path := configFile(t, `{"address":"file:8000"}`)
	for _, app := range []string{"agent", "server"} {
		for _, tc := range []struct {
			name    string
			args    []string
			env     map[string]string
			address string
		}{
			{"short flag", []string{"-c", path}, nil, "file:8000"},
			{"long flag", []string{"-config=" + path}, nil, "file:8000"},
			{"CONFIG wins", []string{"-c=missing.json"}, map[string]string{"CONFIG": path}, "file:8000"},
			{"empty CONFIG disables", []string{"-c=missing.json"}, map[string]string{"CONFIG": ""}, "localhost:8080"},
			{"no file", nil, nil, "localhost:8080"},
		} {
			t.Run(app+"/"+tc.name, func(t *testing.T) {
				prepareConfig(t, tc.args, tc.env)
				if app == "agent" {
					got, err := LoadAgentConfig()
					want := AgentConfig{Address: "http://" + tc.address, ReportInterval: 10, PollInterval: 2, RateLimit: 1}
					if err != nil || got != want {
						t.Fatalf("got %+v, err %v; want %+v", got, err, want)
					}
				} else {
					got, err := LoadServerConfig()
					want := ServerConfig{Address: tc.address, StoreInterval: 300, FileStoragePath: "metrics-storage.json", Restore: true}
					if err != nil || got != want {
						t.Fatalf("got %+v, err %v; want %+v", got, err, want)
					}
				}
			})
		}
	}
}

func TestInvalidConfigFile(t *testing.T) {
	for _, contents := range []string{"", "null", "[]", "{} {}", "{", `{"unknown":1}`, `{"address":42}`, `{"address":null}`, `{"crypto_key":false}`} {
		for _, app := range []string{"agent", "server"} {
			t.Run(app+"/"+contents, func(t *testing.T) {
				prepareConfig(t, []string{"-c", configFile(t, contents)}, nil)
				var err error
				if app == "agent" {
					_, err = LoadAgentConfig()
				} else {
					_, err = LoadServerConfig()
				}
				if err == nil {
					t.Fatal("expected configuration error")
				}
			})
		}
	}
	for _, tc := range []struct{ app, contents string }{
		{"agent", `{"poll_interval":"500ms"}`},
		{"agent", `{"poll_interval":"-1s"}`},
		{"agent", `{"report_interval":"0s"}`},
		{"agent", `{"report_interval":"bad"}`},
		{"agent", `{"poll_interval":2}`},
		{"agent", `{"rate_limit":0}`},
		{"agent", `{"rate_limit":1.5}`},
		{"server", `{"store_interval":"-1s"}`},
		{"server", `{"restore":"false"}`},
	} {
		t.Run(tc.app+"/"+tc.contents, func(t *testing.T) {
			prepareConfig(t, []string{"-c", configFile(t, tc.contents)}, nil)
			var err error
			if tc.app == "agent" {
				_, err = LoadAgentConfig()
			} else {
				_, err = LoadServerConfig()
			}
			if err == nil {
				t.Fatal("expected invalid value error")
			}
		})
	}
	t.Run("missing", func(t *testing.T) {
		prepareConfig(t, []string{"-c", filepath.Join(t.TempDir(), "missing")}, nil)
		if _, err := LoadServerConfig(); err == nil {
			t.Fatal("missing file accepted")
		}
	})
}

func TestStoreFileEnvironmentAlias(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  map[string]string
		path string
	}{
		{"alias", map[string]string{"STORE_FILE": "alias.json"}, "alias.json"},
		{"existing name wins", map[string]string{"STORE_FILE": "alias.json", "FILE_STORAGE_PATH": "existing.json"}, "existing.json"},
		{"empty disables", map[string]string{"STORE_FILE": "alias.json", "FILE_STORAGE_PATH": ""}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prepareConfig(t, []string{"-c", configFile(t, `{"store_file":"file.json"}`)}, tc.env)
			got, err := LoadServerConfig()
			if err != nil || got.FileStoragePath != tc.path || got.FileStorageEnabled != (tc.path != "") {
				t.Fatalf("got %+v, err %v", got, err)
			}
		})
	}
}
