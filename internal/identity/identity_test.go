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
	got, err := NormalizeCode("1234 5678 9012")
	if err != nil || got != "123456789012" {
		t.Fatalf("normalize 12 = %q (%v)", got, err)
	}
	// código antigo de 19 dígitos continua válido
	old, err := NormalizeCode("123 456-789 0123456789")
	if err != nil || old != "1234567890123456789" {
		t.Fatalf("normalize 19 = %q (%v)", old, err)
	}
	if _, err := NormalizeCode("1234567"); err == nil {
		t.Fatal("esperava erro para 7 dígitos")
	}
	if _, err := NormalizeCode("12345678901234567890"); err == nil {
		t.Fatal("esperava erro para 20 dígitos")
	}
}

func TestFormatCode(t *testing.T) {
	if got := FormatCode("123456789012"); got != "1234 5678 9012" {
		t.Fatalf("FormatCode = %q", got)
	}
}

func TestDeriveDeterministic(t *testing.T) {
	const code = "123456789012"
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
	if id1.Code != "1234 5678 9012" {
		t.Fatalf("Code exibido = %q", id1.Code)
	}
	onion, err := DeriveFromCode("1234 5678 9012")
	if err != nil || onion != id1.Onion {
		t.Fatalf("DeriveFromCode = %q (%v)", onion, err)
	}
}
