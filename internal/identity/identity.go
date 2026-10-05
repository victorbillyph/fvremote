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
	"math/big"
	"os"
	"strings"
	"time"

	"filippo.io/edwards25519"

	"github.com/victorbillyph/fvremote/internal/config"
)

// codeLen é o número de dígitos do código (apenas números).
const codeLen = 19

// Identity é a identidade única deste dispositivo.
type Identity struct {
	Code    string // código numérico de 19 dígitos
	Created time.Time

	Onion   string // endereço .onion derivado do código
	TorBlob string // chave ED25519-V3 em base64 (64 bytes: escalar||prf)
	Pub     []byte // chave pública ed25519
}

type diskFormat struct {
	Code    string    `json:"code"`
	Created time.Time `json:"created"`
}

// LoadOrCreate carrega a identidade salva ou cria uma nova na primeira execução.
func LoadOrCreate() (*Identity, error) {
	if err := config.EnsureBase(); err != nil {
		return nil, err
	}
	if b, err := os.ReadFile(config.IdentityPath()); err == nil {
		var d diskFormat
		if err := json.Unmarshal(b, &d); err == nil {
			if code, err := NormalizeCode(d.Code); err == nil {
				return derive(code)
			}
		}
	}
	code, err := newCode()
	if err != nil {
		return nil, err
	}
	id, err := derive(code)
	if err != nil {
		return nil, err
	}
	b, _ := json.MarshalIndent(diskFormat{Code: code, Created: time.Now().UTC()}, "", "  ")
	if err := os.WriteFile(config.IdentityPath(), b, 0o600); err != nil {
		return nil, err
	}
	return id, nil
}

// newCode gera um código numérico aleatório de 19 dígitos.
func newCode() (string, error) {
	limit := new(big.Int).Exp(big.NewInt(10), big.NewInt(codeLen), nil)
	n, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%0*d", codeLen, n), nil
}

// NormalizeCode mantém apenas dígitos e exige exatamente codeLen dígitos.
func NormalizeCode(code string) (string, error) {
	var sb strings.Builder
	for _, r := range code {
		if r >= '0' && r <= '9' {
			sb.WriteRune(r)
		}
	}
	clean := sb.String()
	if len(clean) != codeLen {
		return "", fmt.Errorf("código inválido: esperado %d dígitos, recebido %d", codeLen, len(clean))
	}
	return clean, nil
}

// DeriveFromCode calcula o endereço .onion a partir de um código digitado.
func DeriveFromCode(code string) (onion string, err error) {
	clean, err := NormalizeCode(code)
	if err != nil {
		return "", err
	}
	id, err := derive(clean)
	if err != nil {
		return "", err
	}
	return id.Onion, nil
}

// derive calcula chave e endereço .onion a partir do código canônico.
func derive(code string) (*Identity, error) {
	seed := sha256.Sum256([]byte("fvremote/hs/v2:" + code))
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
		Code:    code,
		Created: time.Now().UTC(),
		Onion:   onion,
		TorBlob: base64.StdEncoding.EncodeToString(blob),
		Pub:     pub,
	}, nil
}
