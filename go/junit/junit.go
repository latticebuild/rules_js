package junit

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func Temporary(destination string) (string, error) {
	file, err := os.CreateTemp(filepath.Dir(destination), ".junit-*.partial")
	if err != nil {
		return "", err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(file.Name())
		return "", err
	}
	return file.Name(), nil
}

func Publish(partial, destination string) error {
	file, err := os.Open(partial)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil || info.Size() == 0 {
		return err
	}
	decoder := xml.NewDecoder(file)
	depth, roots := 0, 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("incomplete JUnit report: %w", err)
		}
		switch token := token.(type) {
		case xml.StartElement:
			if depth == 0 {
				roots++
				if roots != 1 || (token.Name.Local != "testsuites" && token.Name.Local != "testsuite") {
					return errors.New("invalid JUnit report root")
				}
			}
			depth++
		case xml.EndElement:
			depth--
		case xml.CharData:
			if depth == 0 && strings.TrimSpace(string(token)) != "" {
				return errors.New("text outside JUnit report")
			}
		}
	}
	if roots != 1 || depth != 0 {
		return errors.New("incomplete JUnit report")
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(partial, destination)
}
