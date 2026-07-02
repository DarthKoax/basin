package clipboard

import "github.com/atotto/clipboard"

// WriteAll copies the given text to the system clipboard.
func WriteAll(text string) error {
    return clipboard.WriteAll(text)
}

// Write copies the given text to the system clipboard.
func Write(text string) error {
    return clipboard.WriteAll(text)
}

// ReadAll returns the current text from the system clipboard.
func ReadAll() (string, error) {
    return clipboard.ReadAll()
}
