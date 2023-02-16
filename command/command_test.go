package command

import (
	"context"
	"os"
	"testing"

	"github.com/sethvargo/ratchet/parser"
)

const (
	one = `jobs:
  my_job:
    steps:
    - uses: 'actions/checkout@v1'`
	two = `jobs:
  my_job:
    steps:
    - uses: 'actions/setup-go@v1'
    - uses: 'actions/setup-node@v1'`
	four = `jobs:
  my_job:
    steps:
    - uses: 'actions/checkout@v1'
    - uses: 'actions/setup-go@v1'
    - uses: 'actions/setup-node@v1'
    - uses: 'actions/setup-dotnet@v1'`
)

func Test_do(t *testing.T) {
	walkdir := t.TempDir()
	if err := os.WriteFile(walkdir+"/one.yml", []byte(one), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(walkdir+"/two.yml", []byte(two), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(walkdir+"/four.yml", []byte(four), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(walkdir)
	})

	tests := []struct {
		name       string
		path       string
		doerFn     Doer
		flagParser string
		cacheHit   int
		wantErr    bool
	}{
		{
			name:       "pin: if given path is a file",
			path:       walkdir + "/one.yml",
			doerFn:     (&PinCommand{flagConcurrency: 2}).Do,
			flagParser: "actions",
			cacheHit:   1,
			wantErr:    false,
		},
		{
			name:       "pin: if given path is a directory",
			path:       walkdir,
			doerFn:     (&PinCommand{flagConcurrency: 2}).Do,
			flagParser: "actions",
			cacheHit:   4, // 4 unique action refs
			wantErr:    false,
		},
		{
			name:       "update: if given path is a file",
			path:       walkdir + "/one.yml",
			doerFn:     (&UpdateCommand{PinCommand{flagConcurrency: 2}}).Do,
			flagParser: "actions",
			cacheHit:   1,
			wantErr:    false,
		},
		{
			name:       "update: if given path is a directory",
			path:       walkdir,
			doerFn:     (&UpdateCommand{PinCommand{flagConcurrency: 2}}).Do,
			flagParser: "actions",
			cacheHit:   4,
			wantErr:    false,
		},
		{
			name:       "check: if given path is a file",
			path:       walkdir + "/four.yml",
			doerFn:     (&CheckCommand{}).Do,
			flagParser: "actions",
			cacheHit:   0, // no cache hit
			wantErr:    false,
		},
		{
			name:       "check: if given path is a directory",
			path:       walkdir,
			doerFn:     (&CheckCommand{}).Do,
			flagParser: "actions",
			cacheHit:   0,
			wantErr:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := do(context.Background(), tt.path, tt.doerFn, tt.flagParser, 2); (err != nil) != tt.wantErr {
				t.Errorf("do() error = %v, wantErr %v", err, tt.wantErr)
			}
			parser.CacheInvalidate()
		})
	}
}
