package torx

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

type control struct {
	conn net.Conn
	r    *bufio.Reader
	mu   sync.Mutex
}

func dialControl(addr, cookiePath string) (*control, error) {
	var cookie []byte
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		b, err := os.ReadFile(cookiePath)
		if err == nil && len(b) == 32 {
			cookie = b
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if len(cookie) != 32 {
		return nil, fmt.Errorf("cookie do control port não encontrado em %s", cookiePath)
	}

	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, err
	}
	c := &control{conn: conn, r: bufio.NewReader(conn)}

	if _, err := c.cmd("AUTHENTICATE " + hex.EncodeToString(cookie)); err != nil {
		conn.Close()
		return nil, fmt.Errorf("autenticação no control port: %w", err)
	}
	return c, nil
}

// cmd envia um comando e retorna a resposta completa (multilinha).
func (c *control) cmd(command string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, err := fmt.Fprintf(c.conn, "%s\r\n", command); err != nil {
		return "", err
	}
	return readReply(c.r)
}

func readReply(r *bufio.Reader) (string, error) {
	var sb strings.Builder
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return sb.String(), err
		}
		sb.WriteString(line)
		if len(line) >= 4 && line[3] == ' ' {
			break
		}
	}
	return sb.String(), nil
}

func (c *control) close() {
	if c.conn != nil {
		_, _ = fmt.Fprintf(c.conn, "QUIT\r\n")
		_ = c.conn.Close()
	}
}
