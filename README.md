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

Output:

```
Generated passwords:
1: aaaaaaaa
2: baaaaaaa
3: caaaaaaa
4: daaaaaaa
5: eaaaaaaa
6: faaaaaaa
7: gaaaaaaa
8: haaaaaaa
9: iaaaaaaa
10: jaaaaaaa

Batch generated passwords:
1: kaaaaaaa
2: laaaaaaa
3: maaaaaaa
4: naaaaaaa
5: oaaaaaaa

Generated 1026 passwords in 102.893µs
```

There's also a example program that generates a batch of passwords with a
specified character set and length range. See
[examples/wordgen.go](examples/wordgen.go) for details.

```
go run github.com/xiam/wordgen/examples/wordgen -min 1 -max 5

Generating passwords with:
- Character set: abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 (62 characters)
- Length range: 1 to 5 characters
- Theoretical total: 931.2M passwords

Starting password generation...
Progress: 63527771 passwords, 63524042.52 passwords/sec
Progress: 125246501 passwords, 62621035.91 passwords/sec
Progress: 186696237 passwords, 62229455.63 passwords/sec
Progress: 248907786 passwords, 62225149.72 passwords/sec
Progress: 311028514 passwords, 62203700.82 passwords/sec
Progress: 373033361 passwords, 62171095.64 passwords/sec
Progress: 434798773 passwords, 62112262.07 passwords/sec
Progress: 496111864 passwords, 62013427.70 passwords/sec
Progress: 556820106 passwords, 61868101.44 passwords/sec
Progress: 614991279 passwords, 61498875.65 passwords/sec
Progress: 675490253 passwords, 61407468.26 passwords/sec
Progress: 737673075 passwords, 61471966.81 passwords/sec
Progress: 799731690 passwords, 61517514.61 passwords/sec
Progress: 861665466 passwords, 61546577.89 passwords/sec
Progress: 922954289 passwords, 61529521.24 passwords/sec

Generation complete!
Total passwords generated: 931151402
Time taken: 15.132041873s
Generation speed: 61.5M passwords/sec
```

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE.md) file
for details.
