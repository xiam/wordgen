package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/big"
	"os"
	"time"

	"github.com/xiam/wordgen"
)

const (
	bufferSize = uint64(8e6)
	batchSize  = uint64(1e3)
)

func main() {
	charset := flag.String("charset", "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789", "Character set to use for passwords")

	minLen := flag.Uint("min", 1, "Minimum password length")
	maxLen := flag.Uint("max", 5, "Maximum password length")

	printPasswords := flag.Bool("print", false, "Print each generated password (warning: high output volume)")

	// Parse command-line flags
	flag.Parse()

	// Validate inputs
	if *minLen > *maxLen {
		fmt.Println("Error: Minimum length cannot be greater than maximum length")
		os.Exit(1)
	}

	if len(*charset) == 0 {
		fmt.Println("Error: Character set cannot be empty")
		os.Exit(1)
	}

	config := wordgen.Config{
		Charset:    *charset,
		MinLen:     *minLen,
		MaxLen:     *maxLen,
		BufferSize: bufferSize,
	}

	fmt.Printf("Generating passwords with:\n")
	fmt.Printf("- Character set: %s (%d characters)\n", *charset, len(*charset))
	fmt.Printf("- Length range: %d to %d characters\n", *minLen, *maxLen)

	// Calculate the theoretical total
	charsetLen := len(*charset)
	theoretical := big.NewInt(0)
	for length := *minLen; length <= *maxLen; length++ {
		// Calculate charset^length
		lengthCombinations := big.NewInt(1)
		for i := uint(0); i < length; i++ {
			lengthCombinations.Mul(lengthCombinations, big.NewInt(int64(charsetLen)))
		}
		theoretical.Add(theoretical, lengthCombinations)
	}

	fmt.Printf("- Theoretical total: %s passwords\n\n", formatBigNumber(theoretical))

	generator, err := wordgen.NewWordGen(config)
	if err != nil {
		fmt.Printf("Error creating generator: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the generator
	fmt.Println("Starting password generation...")
	errCh, err := generator.Run(ctx)
	if err != nil {
		fmt.Printf("Error starting generator: %v\n", err)
		os.Exit(1)
	}

	// Process passwords in batches for better performance
	batch := make([][]byte, batchSize)

	// Create a ticker to periodically print progress
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// Process passwords until we're done
loop:
	for {
		select {
		case <-ctx.Done():
			// Context cancelled, exit the loop
			break loop

		case err := <-errCh:
			if err != nil {
				if !errors.Is(err, io.EOF) {
					fmt.Printf("Generator failed: %v", err)
					os.Exit(1)
				}
			}

		case <-ticker.C:
			// Print progress every second
			count, duration := generator.Stats()
			if duration > 0 {
				rate := float64(count) / duration.Seconds()
				fmt.Printf("Progress: %d passwords, %.2f passwords/sec\n", count, rate)
			}

		default:
		}

		// Get a batch of passwords
		count, err := generator.Batch(batch)
		if err != nil {
			fmt.Printf("Error getting batch: %v\n", err)
			os.Exit(1)
		}

		if count == 0 {
			break loop
		}

		if *printPasswords {
			for i := 0; i < count; i++ {
				fmt.Printf("%s\n", batch[i])
			}
		}
	}

	// Get final stats
	count, duration := generator.Stats()
	rate := float64(count) / duration.Seconds()

	fmt.Printf("\nGeneration complete!\n")
	fmt.Printf("Total passwords generated: %d\n", count)
	fmt.Printf("Time taken: %v\n", duration)
	fmt.Printf("Generation speed: %s passwords/sec\n", formatNumber(uint64(rate)))
}

// formatNumber formats a uint64 with thousand separators
func formatNumber(n uint64) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	} else if n < 1000000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	} else if n < 1000000000 {
		return fmt.Sprintf("%.1fM", float64(n)/1000000)
	}
	return fmt.Sprintf("%.1fB", float64(n)/1000000000)
}

// formatBigNumber formats a big.Int with thousand separators
func formatBigNumber(n *big.Int) string {
	// If it fits in uint64, use the other formatter
	if n.Cmp(new(big.Int).SetUint64(^uint64(0))) <= 0 {
		return formatNumber(n.Uint64())
	}

	// For really big numbers, use scientific notation
	return n.String()
}
