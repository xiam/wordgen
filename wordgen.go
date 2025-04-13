package wordgen

import (
	"bytes"
	"fmt"
	"io"
	"sync"
	"time"
)

type Config struct {
	Charset      string
	InitialState []byte
	MinLen       uint
	MaxLen       uint
	BufSize      uint
}

type PassGen struct {
	config Config

	charset    []byte
	charsetLen uint

	startTime time.Time
	generated uint64

	buf   [][]byte
	bufMu sync.Mutex

	state    []uint
	stateLen int
	mu       sync.Mutex
}

const (
	defaultCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	defaultMinLen  = 8
	defaultMaxLen  = 10
)

func NewPassGen(config Config) (*PassGen, error) {
	if config.MinLen == 0 {
		config.MinLen = defaultMinLen
	}

	if config.MaxLen == 0 {
		config.MaxLen = defaultMaxLen
	}

	if config.Charset == "" {
		config.Charset = defaultCharset
	}

	if config.MinLen > config.MaxLen {
		return nil, fmt.Errorf("min length cannot be greater than max length")
	}

	p := &PassGen{
		config:     config,
		charset:    []byte(config.Charset),
		charsetLen: uint(len(config.Charset)),
		state:      make([]uint, 0, config.MaxLen),
	}

	// validate charset
	if p.charsetLen == 0 {
		return nil, fmt.Errorf("charset cannot be empty")
	}

	charsetSeen := map[byte]bool{}
	for i, c := range p.charset {
		if charsetSeen[c] {
			return nil, fmt.Errorf("duplicate character in charset at index %d", i)
		}

		charsetSeen[c] = true
	}

	if len(config.InitialState) > 0 {
		state := make([]uint, len(config.InitialState))
		if len(state) < int(config.MinLen) {
			return nil, fmt.Errorf("initial state is less than min length")
		}

		if len(state) > int(config.MaxLen) {
			return nil, fmt.Errorf("initial state is greater than max length")
		}

		for i, c := range config.InitialState {
			n := bytes.IndexByte(p.charset, c)
			if n < 0 {
				return nil, fmt.Errorf("initial state contains character not in charset at index %d", i)
			}
			state[i] = uint(n)
		}

		p.state = state
	}

	// pre-generate buffer
	p.buf = make([][]byte, config.BufSize)
	for i := 0; i < int(config.BufSize); i++ {
		p.buf[i] = make([]byte, 0, config.MaxLen)
	}

	return p, nil
}

func (pg *PassGen) Stats() (uint64, time.Duration) {
	pg.mu.Lock()
	defer pg.mu.Unlock()

	return pg.generated, time.Since(pg.startTime)
}

func (pg *PassGen) Next() ([]byte, error) {
	pg.mu.Lock()
	defer pg.mu.Unlock()

	if pg.stateLen == 0 {
		// initial state
		pg.stateLen = int(pg.config.MinLen)
		pg.state = make([]uint, pg.stateLen)
		pg.startTime = time.Now()
	} else {
		// increment previous state
		for i := 0; ; i++ {
			if i >= pg.stateLen {
				pg.state = append(pg.state, 0)
				pg.stateLen++
				if pg.stateLen > int(pg.config.MaxLen) {
					return nil, io.EOF
				}
				break
			}

			// increment state
			pg.state[i]++

			// check if state is within charset
			if pg.state[i] < pg.charsetLen {
				break
			}

			// keep state within charset
			pg.state[i] = pg.state[i] % pg.charsetLen
		}
	}

	// convert state to password
	buf := make([]byte, pg.stateLen)
	for i := 0; i < pg.stateLen; i++ {
		buf[i] = pg.charset[pg.state[i]]
	}

	// update stats
	pg.generated++

	return buf, nil
}
