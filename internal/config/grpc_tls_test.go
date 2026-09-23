package config

import "testing"

func TestGRPCTLSPrecedence(t *testing.T) {
	for _, app := range []string{"agent", "server"} {
		body := `{"grpc_tls_ca":"file.crt"}`
		flagArgs := []string{"-grpc-tls-ca=flag.crt"}
		env := map[string]string{"GRPC_TLS_CA": "env.crt"}
		if app == "server" {
			body = `{"grpc_tls_cert":"file.crt","grpc_tls_key":"file.key"}`
			flagArgs = []string{"-grpc-tls-cert=flag.crt", "-grpc-tls-key=flag.key"}
			env = map[string]string{"GRPC_TLS_CERT": "env.crt", "GRPC_TLS_KEY": "env.key"}
		}
		path := configFile(t, body)
		for _, tc := range []struct {
			name string
			args []string
			env  map[string]string
			want string
		}{{"file", nil, nil, "file"}, {"flag", flagArgs, nil, "flag"}, {"env", flagArgs, env, "env"}} {
			t.Run(app+"/"+tc.name, func(t *testing.T) {
				prepareConfig(t, append([]string{"-c", path, "-grpc-address=localhost:9090"}, tc.args...), tc.env)
				if app == "agent" {
					cfg, err := LoadAgentConfig()
					if err != nil || cfg.GRPCTLSCA != tc.want+".crt" {
						t.Fatalf("config=%+v err=%v", cfg, err)
					}
				} else {
					cfg, err := LoadServerConfig()
					if err != nil || cfg.GRPCTLSCert != tc.want+".crt" || cfg.GRPCTLSKey != tc.want+".key" {
						t.Fatalf("config=%+v err=%v", cfg, err)
					}
				}
			})
		}
	}
}

func TestGRPCServerRequiresTLSFiles(t *testing.T) {
	for _, args := range [][]string{nil, {"-grpc-tls-cert=test.crt"}, {"-grpc-tls-key=test.key"}} {
		prepareConfig(t, append([]string{"-grpc-address=localhost:9090"}, args...), nil)
		if _, err := LoadServerConfig(); err == nil {
			t.Fatalf("server accepted incomplete TLS settings: %v", args)
		}
	}
	path := configFile(t, `{"grpc_tls_cert":"file.crt","grpc_tls_key":"file.key"}`)
	prepareConfig(t, []string{"-c", path, "-grpc-address=localhost:9090"}, map[string]string{"GRPC_TLS_CERT": ""})
	if _, err := LoadServerConfig(); err == nil {
		t.Fatal("empty environment certificate did not override JSON")
	}
}

func TestGRPCClientEmptyCAUsesSystemRoots(t *testing.T) {
	path := configFile(t, `{"grpc_tls_ca":"file.crt"}`)
	prepareConfig(t, []string{"-c", path, "-grpc-tls-ca=flag.crt"}, map[string]string{"GRPC_TLS_CA": ""})
	cfg, err := LoadAgentConfig()
	if err != nil || cfg.GRPCTLSCA != "" {
		t.Fatalf("config=%+v err=%v", cfg, err)
	}
}
