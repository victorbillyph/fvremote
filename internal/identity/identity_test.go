package identity

import "testing"

func TestDeriveKnownVector(t *testing.T) {
	// Vetor validado contra o comando ADD_ONION do Tor real.
	id, err := derive([]byte("teste-fvremote-123"))
	if err != nil {
		t.Fatal(err)
	}
	const want = "pvzmjwvpwau3cdp55k2za7jrsmmfivtkzdaxmuu3ns5xdpk2wxpvfjqd.onion"
	if id.Onion != want {
		t.Fatalf("onion = %s, quer %s", id.Onion, want)
	}
}

func TestCodeRoundTrip(t *testing.T) {
	raw := make([]byte, 20)
	for i := range raw {
		raw[i] = byte(i * 7)
	}
	code := FormatCode(raw)
	got, err := ParseCode(code)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(raw) {
		t.Fatalf("round trip falhou: %x != %x (code=%s)", got, raw, code)
	}
}

func TestDeriveFromCode(t *testing.T) {
	if _, err := DeriveFromCode("XXXX"); err == nil {
		t.Fatal("esperava erro para código curto")
	}
}
