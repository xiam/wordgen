// Package wordgen provides a password generator that generates passwords based
// on a given character set and length.

package wordgen

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math/bits"
	"sync"
	"time"
)

const (
	defaultCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	defaultMinLen  = 8
	defaultMaxLen  = 10

	minBufferSize = 1
)

type Config struct {
	Charset      string
	InitialState []byte
	MinLen       uint
	MaxLen       uint
	BufferSize   uint64
}

type WordGen struct {
	config Config

	charset    []byte
	charsetLen uint

	startTime time.Time
	endTime   time.Time

	generated uint64

	bufSize uint64
	bufMask uint64

	buf  [][]byte
	bufR uint64
	bufW uint64
	mu   sync.Mutex

	genCond *sync.Cond

	state        []uint
	initialState []uint
	stateLen     int

	running bool
}

// NewWordGen creates a new WordGen instance with the provided configuration.
func NewWordGen(config Config) (*WordGen, error) {
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

	if config.BufferSize < minBufferSize {
		config.BufferSize = minBufferSize
	}

	pg := &WordGen{
		config:       config,
		charset:      []byte(config.Charset),
		charsetLen:   uint(len(config.Charset)),
		state:        make([]uint, 0, config.MaxLen),
		initialState: make([]uint, config.MaxLen),
	}

	// validate charset
	if pg.charsetLen == 0 {
		return nil, fmt.Errorf("charset cannot be empty")
	}

	charsetSeen := map[byte]bool{}
	for i, c := range pg.charset {
		if charsetSeen[c] {
			return nil, fmt.Errorf("duplicate character in charset at index %d", i)
		}

		charsetSeen[c] = true
	}

	if len(config.InitialState) > 0 {
		if len(config.InitialState) < int(config.MinLen) {
			return nil, fmt.Errorf("initial state is less than min length")
		}

		if len(config.InitialState) > int(config.MaxLen) {
			return nil, fmt.Errorf("initial state is greater than max length")
		}

		for i, c := range config.InitialState {
			n := bytes.IndexByte(pg.charset, c)
			if n < 0 {
				return nil, fmt.Errorf("initial state contains character not in charset (%q) at index %d", c, i)
			}

			pg.initialState[i] = uint(n)
		}
		pg.initialState = pg.initialState[:len(config.InitialState)]
	} else {
		pg.initialState = pg.initialState[:config.MinLen]
	}

	// adjust actual buffer size to be a power of two so that we can use bitwise
	// operations to calculate the index in the buffer instead of using modulo
	// operator
	pg.bufSize = roundToNearestPowerOfTwo(config.BufferSize)
	pg.bufMask = pg.bufSize - 1

	// allocate buffer
	pg.buf = make([][]byte, pg.bufSize)
	for i := 0; i < int(pg.bufSize); i++ {
		pg.buf[i] = make([]byte, 0, config.MaxLen)
	}

	pg.genCond = sync.NewCond(&pg.mu)

	return pg, nil
}

// Run starts the password generator. It will generate passwords in the
// background and store them in a buffer. The buffer size is determined by the
// BufferSize parameter in the configuration.
func (pg *WordGen) Run(ctx context.Context) (<-chan error, error) {
	errCh := make(chan error, 1)

	pg.mu.Lock()
	if pg.running {
		pg.mu.Unlock()
		return nil, fmt.Errorf("generator is already running")
	}

	// reset state
	pg.startTime = time.Now()
	pg.endTime = time.Time{}

	pg.state = make([]uint, len(pg.initialState), pg.config.MaxLen)

	// copy initial state
	copy(pg.state, pg.initialState)

	pg.stateLen = len(pg.state)
	pg.bufR = 0
	pg.bufW = 0

	pg.running = true

	pg.mu.Unlock()

	go func() {
		<-ctx.Done()
		pg.mu.Lock()
		pg.running = false
		pg.mu.Unlock()
	}()

	go func() {
		err := pg.runGenerator()

		pg.mu.Lock()
		pg.endTime = time.Now()
		pg.running = false
		pg.mu.Unlock()

		errCh <- err

		close(errCh)
	}()

	return errCh, nil
}

// Stats returns the number of generated passwords and the time taken to
// generate them.
func (pg *WordGen) Stats() (uint64, time.Duration) {
	pg.mu.Lock()
	generated := pg.generated
	startTime := pg.startTime
	endTime := pg.endTime
	pg.mu.Unlock()

	if startTime.IsZero() {
		return 0, 0
	}

	if endTime.IsZero() {
		return generated, time.Since(startTime)
	}

	return generated, endTime.Sub(startTime)
}

// Next generates the next password and returns it as a byte slice.
func (pg *WordGen) Next() ([]byte, error) {
	var idx uint64
	var buf []byte

	pg.mu.Lock()

	if pg.bufR >= pg.bufW {
		// buffer is empty, check if generator is still running
		if !pg.running {
			pg.mu.Unlock()

			return nil, io.EOF
		}

		// wait for the generator to fill the buffer
		pg.genCond.Wait()

		// after waking up, check again if we have data
		if pg.bufR >= pg.bufW {
			pg.mu.Unlock()

			// still no data, generator must have stopped
			return nil, io.EOF
		}
	}

	idx = pg.bufR & pg.bufMask
	buf = pg.buf[idx]
	pg.bufR++

	pg.mu.Unlock()

	return buf, nil
}

// Batch generates a batch of passwords and returns the number of generated
// passwords and an error if any. The passwords are returned in the provided
// slice. The slice must be large enough to hold the generated passwords.
func (pg *WordGen) Batch(words [][]byte) (int, error) {
	if len(words) == 0 {
		return 0, nil
	}

	pg.mu.Lock()

	count := 0
	for i := 0; i < len(words); i++ {
		// Check if there are passwords available in the buffer
		if pg.bufR >= pg.bufW {
			// buffer is empty, check if generator is still running
			if !pg.running {
				// generator has stopped, return what we've got so far
				pg.mu.Unlock()
				return count, nil
			}

			// wait for the generator to fill the buffer
			pg.genCond.Wait()

			// after waking up, check again if we have data
			if pg.bufR >= pg.bufW {
				// still no data, generator must have stopped
				pg.mu.Unlock()
				return count, nil
			}
		}

		// get the next password from the buffer
		idx := pg.bufR & pg.bufMask
		src := pg.buf[idx]

		// make sure the destination slice has enough capacity
		if cap(words[i]) < len(src) {
			words[i] = make([]byte, len(src))
		} else {
			words[i] = words[i][:len(src)]
		}

		// copy the password to the destination slice
		copy(words[i], src)

		// increment read index
		pg.bufR++
		count++
	}
	pg.mu.Unlock()

	return count, nil
}

// Stop stops the password generator.
func (pg *WordGen) Stop() {
	// stop the generator
	pg.mu.Lock()
	pg.running = false
	pg.mu.Unlock()
}

func (pg *WordGen) runGenerator() error {
	var idx uint64
	var buf *[]byte

	for {
		pg.mu.Lock()

		if !pg.running {
			pg.mu.Unlock()
			return nil
		}

		if pg.bufW-pg.bufR >= pg.bufSize {
			pg.mu.Unlock()

			time.Sleep(10 * time.Millisecond)

			continue
		}

		idx = pg.bufW & pg.bufMask

		buf = &pg.buf[idx]

		// resize buffer to state length
		*buf = (*buf)[:pg.stateLen]

		// fill buffer with charset characters
		for i := 0; i < pg.stateLen; i++ {
			(*buf)[i] = pg.charset[pg.state[i]]
		}

		// update write index
		pg.bufW = pg.bufW + 1
		pg.generated++

		pg.genCond.Signal()

		// update next state
		if err := pg.nextState(); err != nil {
			pg.mu.Unlock()
			if errors.Is(err, io.EOF) {
				// EOF means we have generated all passwords
				// and we can stop the generator
				return nil
			}

			return fmt.Errorf("generateNext: %w", err)
		}
		pg.mu.Unlock()
	}

	return nil
}

func (pg *WordGen) nextState() error {
	// increment previous state
	for i := 0; ; i++ {
		if i >= pg.stateLen {
			pg.stateLen++
			if pg.stateLen > int(pg.config.MaxLen) {
				return io.EOF
			}
			pg.state = append(pg.state, 0)
			break
		}

		// increment state
		pg.state[i]++

		// check if state is within charset
		if pg.state[i] < pg.charsetLen {
			break
		}

		// keep state within charset
		pg.state[i] = 0
	}

	return nil
}

func roundToNearestPowerOfTwo(n uint64) uint64 {
	if n == 0 {
		return 1
	}

	if n&(n-1) == 0 {
		// n is already a power of two
		return n
	}

	// find the next and previous powers of 2
	next := uint64(1) << bits.Len64(n)
	prev := next >> 1

	// decide which one is closer
	if n-prev < next-n {
		return prev
	}

	return next
}
