package identity

import "testing"

func TestNewCode(t *testing.T) {
	for i := 0; i < 50; i++ {
		c, err := newCode()
		if err != nil {
			t.Fatal(err)
		}
		if len(c) != codeLen {
			t.Fatalf("código %q tem %d dígitos", c, len(c))
		}
		clean, err := NormalizeCode(c)
		if err != nil || clean != c {
			t.Fatalf("código gerado não normaliza: %q (%v)", c, err)
		}
	}
}

func TestNormalizeCode(t *testing.T) {
	got, err := NormalizeCode("123 456-789 0123456789")
	if err != nil {
		t.Fatal(err)
	}
	if got != "1234567890123456789" {
		t.Fatalf("normalize = %q", got)
	}
	if _, err := NormalizeCode("1234"); err == nil {
		t.Fatal("esperava erro para código curto")
	}
}

func TestDeriveDeterministic(t *testing.T) {
	const code = "1234567890123456789"
	id1, err := derive(code)
	if err != nil {
		t.Fatal(err)
	}
	id2, err := derive(code)
	if err != nil {
		t.Fatal(err)
	}
	if id1.Onion != id2.Onion || id1.TorBlob != id2.TorBlob {
		t.Fatal("derivação não é determinística")
	}
	onion, err := DeriveFromCode("123 456 789 012 345 678 9")
	if err != nil || onion != id1.Onion {
		t.Fatalf("DeriveFromCode = %q (%v)", onion, err)
	}
}
