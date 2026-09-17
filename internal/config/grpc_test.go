package config

import "testing"

func TestGRPCConfig(t *testing.T) {
	path := configFile(t, `{"grpc_address":"file:9090"}`)
	tests := []struct {
		name string
		args []string
		env  map[string]string
		want string
	}{
		{"default", nil, nil, ""},
		{"file", []string{"-c", path}, nil, "file:9090"},
		{"flag", []string{"-c", path, "-grpc-address=flag:9090"}, nil, "flag:9090"},
		{"env", []string{"-c", path, "-grpc-address=flag:9090"}, map[string]string{"GRPC_ADDRESS": "env:9090"}, "env:9090"},
		{"empty", []string{"-c", path}, map[string]string{"GRPC_ADDRESS": ""}, ""},
	}
	for _, kind := range []string{"agent", "server"} {
		for _, tc := range tests {
			t.Run(kind+"/"+tc.name, func(t *testing.T) {
				prepareConfig(t, tc.args, tc.env)
				var got string
				if kind == "agent" {
					cfg, err := LoadAgentConfig()
					if err != nil {
						t.Fatal(err)
					}
					got = cfg.GRPCAddress
				} else {
					cfg, err := LoadServerConfig()
					if err != nil {
						t.Fatal(err)
					}
					got = cfg.GRPCAddress
				}
				if got != tc.want {
					t.Fatalf("got %q want %q", got, tc.want)
				}
			})
		}
	}
}

func TestGRPCRejectsIgnoredSecuritySettings(t *testing.T) {
	for _, kind := range []string{"agent", "server"} {
		for _, key := range []string{"KEY", "CRYPTO_KEY"} {
			t.Run(kind+"/"+key, func(t *testing.T) {
				prepareConfig(t, []string{"-grpc-address=localhost:9090"}, map[string]string{key: "configured"})
				var err error
				if kind == "agent" {
					_, err = LoadAgentConfig()
				} else {
					_, err = LoadServerConfig()
				}
				if err == nil {
					t.Error("gRPC silently ignored security settings")
				}
			})
		}
	}
}
