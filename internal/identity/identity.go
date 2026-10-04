package identity

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha3"
	"crypto/sha512"
	"encoding/base32"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"filippo.io/edwards25519"

	"github.com/victorbillyph/fvremote/internal/config"
)

// crockford é o alfabeto base32 de Crockford (sem I, L, O, U).
const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// Identity é a identidade única deste dispositivo.
type Identity struct {
	CodeBytes []byte
	Created   time.Time

	Code    string // código amigável (exibido ao usuário)
	Onion   string // endereço .onion derivado do código
	TorBlob string // chave ED25519-V3 em base64 (64 bytes: escalar||prf)
	Pub     []byte // chave pública ed25519
}

type diskFormat struct {
	CodeBytes []byte    `json:"code_bytes"`
	Created   time.Time `json:"created"`
}

// LoadOrCreate carrega a identidade salva ou cria uma nova na primeira execução.
func LoadOrCreate() (*Identity, error) {
	if err := config.EnsureBase(); err != nil {
		return nil, err
	}
	if b, err := os.ReadFile(config.IdentityPath()); err == nil {
		var d diskFormat
		if err := json.Unmarshal(b, &d); err == nil && len(d.CodeBytes) > 0 {
			return derive(d.CodeBytes)
		}
	}
	raw := make([]byte, 20) // 160 bits -> 32 caracteres
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	id, err := derive(raw)
	if err != nil {
		return nil, err
	}
	b, _ := json.MarshalIndent(diskFormat{CodeBytes: raw, Created: time.Now().UTC()}, "", "  ")
	if err := os.WriteFile(config.IdentityPath(), b, 0o600); err != nil {
		return nil, err
	}
	return id, nil
}

// derive calcula código, chave e endereço .onion a partir dos bytes do código.
func derive(codeBytes []byte) (*Identity, error) {
	seed := sha256.Sum256(append([]byte("fvremote/hs/v1"), codeBytes...))
	h := sha512.Sum512(seed[:])

	var scalar [32]byte
	copy(scalar[:], h[:32])
	scalar[0] &= 248
	scalar[31] &= 63
	scalar[31] |= 64
	prf := h[32:]

	s, err := edwards25519.NewScalar().SetBytesWithClamping(scalar[:])
	if err != nil {
		return nil, err
	}
	pub := edwards25519.NewIdentityPoint().ScalarBaseMult(s).Bytes()

	blob := append(append([]byte{}, scalar[:]...), prf...)

	ver := byte(0x03)
	sum := sha3.Sum256(append(append([]byte(".onion checksum"), pub...), ver))
	addr := append(append(append([]byte{}, pub...), sum[:2]...), ver)
	onion := strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(addr)) + ".onion"

	return &Identity{
		CodeBytes: codeBytes,
		Created:   time.Now().UTC(),
		Code:      FormatCode(codeBytes),
		Onion:     onion,
		TorBlob:   base64.StdEncoding.EncodeToString(blob),
		Pub:       pub,
	}, nil
}

// DeriveFromCode calcula o endereço .onion a partir de um código digitado.
func DeriveFromCode(code string) (onion string, err error) {
	raw, err := ParseCode(code)
	if err != nil {
		return "", err
	}
	id, err := derive(raw)
	if err != nil {
		return "", err
	}
	return id.Onion, nil
}

// FormatCode converte bytes em um código legível: XXXX-XXXX-... (8 grupos).
func FormatCode(raw []byte) string {
	enc := crockfordEncode(raw)
	var groups []string
	for i := 0; i < len(enc); i += 4 {
		end := i + 4
		if end > len(enc) {
			end = len(enc)
		}
		groups = append(groups, enc[i:end])
	}
	return strings.Join(groups, "-")
}

// ParseCode aceita o código com ou sem separadores, normaliza maiúsculas e
// caracteres ambíguos (I/L->1, O->0).
func ParseCode(code string) ([]byte, error) {
	clean := strings.ToUpper(code)
	clean = strings.NewReplacer("-", "", " ", "", "\t", "", "\n", "", "\r", "").Replace(clean)
	clean = strings.NewReplacer("I", "1", "L", "1", "O", "0").Replace(clean)
	if len(clean) != 32 {
		return nil, fmt.Errorf("código inválido: esperado 32 caracteres, recebido %d", len(clean))
	}
	return crockfordDecode(clean)
}

// crockfordEncode codifica bytes em base32 de Crockford (5 bits por caractere).
func crockfordEncode(raw []byte) string {
	var out []byte
	var buffer uint32
	var bits uint
	for _, b := range raw {
		buffer = (buffer << 8) | uint32(b)
		bits += 8
		for bits >= 5 {
			bits -= 5
			idx := (buffer >> bits) & 0x1f
			out = append(out, crockford[idx])
		}
	}
	if bits > 0 {
		idx := (buffer << (5 - bits)) & 0x1f
		out = append(out, crockford[idx])
	}
	return string(out)
}

// crockfordDecode decodifica o texto Crockford (sem padding) em bytes.
func crockfordDecode(s string) ([]byte, error) {
	var out []byte
	var buffer uint32
	var bits uint
	for _, c := range s {
		idx := strings.IndexRune(crockford, c)
		if idx < 0 {
			return nil, fmt.Errorf("caractere inválido no código: %q", c)
		}
		buffer = (buffer << 5) | uint32(idx)
		bits += 5
		if bits >= 8 {
			bits -= 8
			out = append(out, byte((buffer>>bits)&0xff))
		}
	}
	return out, nil
}
