package ethereum

import "testing"

func TestEIP55Checksum(t *testing.T) {
	// Official EIP-55 test vectors from
	// https://github.com/ethereum/EIPs/blob/master/EIPS/eip-55.md
	tests := []struct {
		lower string
		want  string
	}{
		{"52908400098527886e0f7030069857d2e4169ee7", "52908400098527886E0F7030069857D2E4169EE7"},
		{"8617e340b3d01fa5f11f306f4090fd50e238070d", "8617E340B3D01FA5F11F306F4090FD50E238070D"},
		{"de709f2102306220921060314715629080e2fb77", "de709f2102306220921060314715629080e2fb77"},
		{"27b1fdb04752bbc536007a920d24acb045561c26", "27b1fdb04752bbc536007a920d24acb045561c26"},
		{"5aaeb6053f3e94c9b9a09f33669435e7ef1beaed", "5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed"},
		{"fb6916095ca1df60bb79ce92ce3ea74c37c5d359", "fB6916095ca1df60bB79Ce92cE3Ea74c37c5d359"},
		{"dbf03b407c01e7cd3cbea99509d93f8dddc8c6fb", "dbF03B407c01E7cD3CBea99509d93f8DDDC8C6FB"},
		{"d1220a0cf47c7b9be7a2e6ba89f429762e7b9adb", "D1220A0cf47c7B9Be7A2E6BA89F429762e7b9aDb"},
	}
	for _, tt := range tests {
		if got := EIP55Checksum(tt.lower); got != tt.want {
			t.Errorf("EIP55Checksum(%q) = %q, want %q", tt.lower, got, tt.want)
		}
	}
}
