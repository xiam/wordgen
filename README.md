# wordgen

`wordgen` is a Go library to generate words based on different parameters.

A common use case is to generate random words for testing or security purposes,
such as generating passwords.

The library allocates a buffer of a given size and fills it with the generated
words, so that consumers always have available words to operate with.

Here's a simple example demonstrating how to use the `wordgen` package to
generate passwords:

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/xiam/wordgen"
)

func main() {
    // Create a configuration with custom settings
    config := wordgen.Config{
        Charset:    "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*",
        MinLen:     8,
        MaxLen:     12,
        BufferSize: 1000,
    }

    // Initialize the password generator
    generator, err := wordgen.NewWordGen(config)
    if err != nil {
        log.Fatalf("Failed to create password generator: %v", err)
    }

    // Create a context with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    // Start the generator
    errCh, err := generator.Run(ctx)
    if err != nil {
        log.Fatalf("Failed to start generator: %v", err)
    }

    // Generate and print 10 passwords
    fmt.Println("Generated passwords:")
    for i := 0; i < 10; i++ {
        password, err := generator.Next()
        if err != nil {
            log.Fatalf("Failed to get next password: %v", err)
        }
        fmt.Printf("%d: %s\n", i+1, string(password))
    }

    // Generate a batch of passwords
    batch := make([][]byte, 5)
    count, err := generator.Batch(batch)
    if err != nil {
        log.Fatalf("Failed to get batch of passwords: %v", err)
    }

    fmt.Println("\nBatch generated passwords:")
    for i := 0; i < count; i++ {
        fmt.Printf("%d: %s\n", i+1, string(batch[i]))
    }

    // Get statistics
    generated, duration := generator.Stats()
    fmt.Printf("\nGenerated %d passwords in %v\n", generated, duration)

    // Stop the generator
    generator.Stop()

    // Check for any errors from the generator
    if err := <-errCh; err != nil {
        log.Fatalf("Generator error: %v", err)
    }
}
```

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE.md) file
for details.
