package wordgen

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWordGenNext(t *testing.T) {
	t.Run("all 5-letter words", func(t *testing.T) {
		wordgen, err := NewWordGen(Config{
			MinLen:     5,
			MaxLen:     5,
			BufferSize: 1e6,
		})
		require.NoError(t, err)

		errCh, err := wordgen.Run(context.TODO())
		require.NoError(t, err)

		var lastWord string

		{
			for i := 0; i < 1e9; i++ {
				word, err := wordgen.Next()
				if i%1e8 == 0 {
					generated, elapsed := wordgen.Stats()
					t.Logf("word: %s", word)
					t.Logf("generated: %d, elapsed: %s, word/s: %f", generated, elapsed, float64(generated)/elapsed.Seconds())
				}

				if err != nil {
					if errors.Is(err, io.EOF) {
						t.Logf("last word: %s", lastWord)
						t.Logf("generated: %d", wordgen.generated)
						break
					}
					require.NoError(t, err)
				}
				lastWord = string(word)
			}
		}

		generated, _ := wordgen.Stats()
		require.Equal(t, 916132832, int(generated))

		t.Logf("last word: %s", lastWord)

		wordgen.Stop()

		err = <-errCh
		require.NoError(t, err)
	})

	t.Run("test cases", func(t *testing.T) {
		tests := []struct {
			name          string
			config        Config
			expectedWords []string
			expectEOF     bool
		}{
			{
				name: "Basic operation",
				config: Config{
					Charset:    "abc",
					MinLen:     1,
					MaxLen:     2,
					BufferSize: 10,
				},
				expectedWords: []string{"a", "b", "c", "aa", "ba", "ca", "ab", "bb", "cb"},
			},
			{
				name: "Custom initial state",
				config: Config{
					Charset:      "abc",
					InitialState: []byte("ba"),
					MinLen:       1,
					MaxLen:       2,
					BufferSize:   10,
				},
				expectedWords: []string{"ba", "ca", "ab", "bb", "cb"},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				pg, err := NewWordGen(tt.config)
				if err != nil {
					t.Fatalf("Failed to create WordGen: %v", err)
				}

				// If there's a setup function, run it
				// Otherwise start the generator normally
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				errCh, err := pg.Run(ctx)
				if err != nil {
					t.Fatalf("Failed to run generator: %v", err)
				}

				// Monitor for errors from the generator
				go func() {
					if err := <-errCh; err != nil {
						t.Errorf("Generator error: %v", err)
					}
				}()

				// Read words until we get an error or have read all expected words
				var words []string
				for i := 0; i < len(tt.expectedWords)+1 || (tt.expectEOF && len(words) == 0); i++ {
					word, err := pg.Next()
					if err != nil {
						if err == io.EOF && tt.expectEOF {
							// Expected EOF
							break
						}
						t.Fatalf("Unexpected error from Next(): %v", err)
					}

					words = append(words, string(word))

					// If we've read all expected words and don't expect EOF, we're done
					if len(words) >= len(tt.expectedWords) && !tt.expectEOF {
						break
					}
				}

				// If we expect EOF, try one more read to confirm we get EOF
				if tt.expectEOF {
					_, err := pg.Next()
					if err != io.EOF {
						t.Errorf("Expected EOF, got: %v", err)
					}
				}

				// Verify we got the expected words
				if len(words) != len(tt.expectedWords) {
					t.Errorf("Expected %d words, got %d", len(tt.expectedWords), len(words))
				}

				// Check each word matches expected
				for i := 0; i < len(words) && i < len(tt.expectedWords); i++ {
					if words[i] != tt.expectedWords[i] {
						t.Errorf("Word %d: expected %q, got %q", i, tt.expectedWords[i], words[i])
					}
				}
			})
		}
	})

	t.Run("concurrent access", func(t *testing.T) {
		// Test concurrent access to Next()
		pg, err := NewWordGen(Config{
			Charset:    "abc",
			MinLen:     1,
			MaxLen:     5,
			BufferSize: 100,
		})
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		errCh, err := pg.Run(ctx)
		require.NoError(t, err)

		// Monitor for errors from the generator
		go func() {
			err := <-errCh
			require.NoError(t, err)
		}()

		// Have multiple goroutines call Next() concurrently
		const numGoroutines = 5
		const wordsPerGoroutine = 20

		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()

				for j := 0; j < wordsPerGoroutine; j++ {
					word, err := pg.Next()
					require.NoError(t, err, "unexpected error: %v", err)
					assert.GreaterOrEqual(t, len(word), 1, "word length should be >= 1")
					assert.LessOrEqual(t, len(word), 5, "word length should be <= 5")
				}
			}()
		}

		wg.Wait()
	})
}

func TestWordGenBatch(t *testing.T) {
	wordgen, err := NewWordGen(Config{
		MinLen:     3,
		MaxLen:     5,
		BufferSize: 1000,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	errCh, err := wordgen.Run(ctx)
	require.NoError(t, err)
	defer func() {
		wordgen.Stop()
		err := <-errCh
		require.NoError(t, err)
	}()

	batchSizes := []int{1, 10, 100}
	for _, size := range batchSizes {
		t.Run(fmt.Sprintf("batch_size_%d", size), func(t *testing.T) {
			batch := make([][]byte, size)
			n, err := wordgen.Batch(batch)
			require.NoError(t, err)
			require.Len(t, batch, size, "Batch should return exactly the requested number of words")
			require.Equal(t, n, size, "Batch should return the requested number of words")

			for i, word := range batch {
				wordLen := uint(len(word))

				require.GreaterOrEqual(t, wordLen, wordgen.config.MinLen, "Word %d length should be >= MinLen", i)
				require.LessOrEqual(t, wordLen, wordgen.config.MaxLen, "Word %d length should be <= MaxLen", i)
			}

			uniqueWords := make(map[string]struct{}, len(batch))
			for _, word := range batch {
				uniqueWords[string(word)] = struct{}{}
			}
			require.Len(t, uniqueWords, len(batch), "All words in batch should be unique")
		})
	}
}
