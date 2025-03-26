package linter

// Violation represents an instance of a linting violation.
type Violation struct {
	Filename string
	Contents string
	Line     int
	Column   int
}
