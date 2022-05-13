package atomic

import (
	"os"
	"strings"
	"testing"
)

func TestWrite(t *testing.T) {
	t.Parallel()

	src, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := src.Close(); err != nil {
		t.Fatal(err)
	}

	dst, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := dst.Close(); err != nil {
		t.Fatal(err)
	}

	if err := Write(src.Name(), dst.Name(), strings.NewReader("hello")); err != nil {
		t.Fatal(err)
	}

	f, err := os.ReadFile(dst.Name())
	if err != nil {
		t.Fatal(err)
	}

	if got, want := string(f), "hello"; got != want {
		t.Errorf("expected %q to be %q", got, want)
	}
}
