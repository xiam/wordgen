package wordgen

import (
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewPassGen(t *testing.T) {
	wordgen, err := NewPassGen(Config{
		MinLen:  5,
		MaxLen:  5,
		BufSize: 1e7,
	})
	require.NoError(t, err)

	{
		var lastWord string
		for i := 0; i < 1e10; i++ {
			word, err := wordgen.Next()
			if i%1e8 == 0 {
				generated, elapsed := wordgen.Stats()
				t.Logf("word: %s", word)
				t.Logf("generated: %d, elapsed: %s, word/s: %f", generated, elapsed, float64(generated)/elapsed.Seconds())
			}
			if err != nil {
				if err == io.EOF {
					t.Logf("last word: %s", lastWord)
					t.Logf("generated: %d", wordgen.generated)
					break
				}
				require.NoError(t, err)
			}
			lastWord = string(word)
		}
	}
}
