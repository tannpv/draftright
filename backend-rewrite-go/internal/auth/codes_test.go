package auth

import "testing"

func TestGenerateCode_SixDigitsPadded(t *testing.T) {
	for i := 0; i < 200; i++ {
		c := generateCode()
		if len(c) != 6 {
			t.Fatalf("len %q", c)
		}
		for _, r := range c {
			if r < '0' || r > '9' {
				t.Fatalf("non-digit %q", c)
			}
		}
	}
}
