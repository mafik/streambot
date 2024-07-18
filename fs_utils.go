package main

import (
	"os"
	"strings"
)

// Reads a file and returns it as a string. The returned value is trimmed from any whitespace.
func ReadStringFromFile(filename string) (string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}
	str := string(data)
	return strings.TrimSpace(str), nil
}

func WriteStringToFile(filename, data string) error {
	return os.WriteFile(filename, []byte(data), 0644)
}

func AppendToFile(filename, text string) (err error) {
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err = f.WriteString(text); err != nil {
		return err
	}
	return nil
}
