package ferrite

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/dogmatiq/ferrite/internal/table"
	"github.com/dogmatiq/ferrite/schema"
	"golang.org/x/exp/slices"
)

// ValidateEnvironment validates all environment variables.
func ValidateEnvironment() {
	if result, ok := validate(); !ok {
		io.WriteString(output, result)
		exit(1)
	}
}

// Register adds a validator to the global validation system.
func Register(v Validator) {
	validatorsM.Lock()
	validators = append(validators, v)
	validatorsM.Unlock()
}

// Validator is an interface used to validate environment variables
type Validator interface {
	// Validate validates the environment variable.
	Validate() []ValidationResult
}

// ValidationResult is the result of validating an environment variable.
type ValidationResult struct {
	// Name is the name of the environment variable.
	Name string

	// Description is a human-readable description of the environment variable.
	Description string

	// Schema describes the valid values for this environment variable.
	Schema schema.Schema

	// DefaultValue is the environment variable's default value, rendered as it
	// should be displayed in the console.
	//
	// This is not necessarily equal to a raw environment variable value. For
	// example, StringSpec renders strings with surrounding quotes.
	//
	// It must be non-empty if the environment variable has a default value;
	// otherwise it must be empty.
	DefaultValue string

	// ExplicitValue is the environment variable's value as captured from the
	// environment, rendered as it should be displayed in the console.
	//
	// This is not necessarily equal to the raw environment variable value. For
	// example, StringSpec renders strings with surrounding quotes.
	ExplicitValue string

	// UsingDefault is true if the environment variable's default value
	// would be retuned by the specs Value() method.
	UsingDefault bool

	// Error is an error describing why the validation failed.
	//
	// If it is nil, the validation is considered successful.
	Error error
}

var (
	// validators is a global set of validators that are invoked by
	// ValidateEnvironment().
	validatorsM sync.Mutex
	validators  []Validator

	// output is the writer to which the validation result is written.
	output io.Writer = os.Stderr

	// exit is called to exit the process when validation fails.
	exit = os.Exit
)

// validate parses and validates all environment variables.
func validate() (string, bool) {
	validatorsM.Lock()
	defer validatorsM.Unlock()

	var results []ValidationResult
	ok := true

	for _, s := range validators {
		for _, res := range s.Validate() {
			if res.Error != nil {
				ok = false
			}

			results = append(results, res)
		}
	}

	return renderResults(results), ok
}

// inputType returns a strring describing a variable's valid input as any
// value of type T.
func inputType[T any]() string {
	var zero T
	return fmt.Sprintf("[%T]", zero)
}

// inputList returns a string describing a variable's vaild input as a list of
// accepted values.
func inputList(values ...string) string {
	return strings.Join(values, "|")
}

const (
	// valid is the icon displayed next to valid environment variables.
	valid = "✓"

	// invalid is the icon displayed next to invalid environment variables.
	invalid = "✗"

	// chevron is the icon used to draw attention to invalid environment
	// variables.
	chevron = "❯"
)

// renderResults renders a set of validation results as a human-readable string.
func renderResults(results []ValidationResult) string {
	slices.SortFunc(
		results,
		func(a, b ValidationResult) bool {
			return a.Name < b.Name
		},
	)

	var t table.Table

	for _, v := range results {
		name := " "
		if v.Error != nil {
			name += chevron
		} else {
			name += " "
		}
		name += " " + v.Name

		renderer := &validateSchemaRenderer{}
		v.Schema.AcceptVisitor(renderer)

		input := renderer.Output.String()
		if v.DefaultValue != "" {
			input += " = " + v.DefaultValue
		}

		status := ""
		if v.Error != nil {
			status += invalid + " " + v.Error.Error()
		} else if v.UsingDefault {
			status += valid + " using default value"
		} else {
			status += valid + " set to " + v.ExplicitValue
		}

		t.AddRow(name, input, v.Description, status)
	}

	return "ENVIRONMENT VARIABLES:\n" + t.String()
}

type validateSchemaRenderer struct {
	Output strings.Builder
}

func (r *validateSchemaRenderer) VisitOneOf(s schema.OneOf) {
	for i, c := range s {
		if i > 0 {
			r.Output.WriteString("|")
		}

		c.AcceptVisitor(r)
	}
}

func (r *validateSchemaRenderer) VisitLiteral(s schema.Literal) {
	r.Output.WriteString(string(s))
}

func (r *validateSchemaRenderer) VisitType(s schema.TypeSchema) {
	fmt.Fprintf(&r.Output, "[%s]", s.Type)
}

func (r *validateSchemaRenderer) VisitRange(s schema.Range) {
	if s.Min != "" && s.Max != "" {
		fmt.Fprintf(&r.Output, "(%s..%s)", s.Min, s.Max)
	} else if s.Max != "" {
		fmt.Fprintf(&r.Output, "(...%s)", s.Max)
	} else {
		fmt.Fprintf(&r.Output, "(%s...)", s.Min)
	}
}
