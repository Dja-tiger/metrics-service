package config

import "testing"

func TestTrustedSubnetPrecedence(t *testing.T) {
	path := configFile(t, `{"trusted_subnet":"10.0.0.0/8"}`)
	for _, tc := range []struct {
		name string
		args []string
		env  map[string]string
		want string
	}{
		{"default", nil, nil, ""},
		{"JSON", []string{"-c", path}, nil, "10.0.0.0/8"},
		{"flag", []string{"-c", path, "-t=192.168.0.0/16"}, nil, "192.168.0.0/16"},
		{"env", []string{"-c", path, "-t=192.168.0.0/16"}, map[string]string{"TRUSTED_SUBNET": "2001:db8::/32"}, "2001:db8::/32"},
		{"empty flag disables", []string{"-c", path, "-t="}, nil, ""},
		{"empty env disables", []string{"-c", path, "-t=192.168.0.0/16"}, map[string]string{"TRUSTED_SUBNET": ""}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prepareConfig(t, tc.args, tc.env)
			cfg, err := LoadServerConfig()
			if err != nil {
				t.Fatal(err)
			}
			if cfg.TrustedSubnet != tc.want {
				t.Fatalf("got %q want %q", cfg.TrustedSubnet, tc.want)
			}
		})
	}
}
