package code

import (
	"regexp"

	"github.com/smacker/go-tree-sitter/python"
)

func Python() *Language {
	return &Language{
		Delims: regexp.MustCompile(`#\s?|\s*"""\s*|\s*'''\s*`),
		Parser: python.GetLanguage(),
		Queries: []string{
			`(comment)+ @comment`,
			// Function docstring
			`((function_definition
  body: (block . (expression_statement (string) @docstring)))
 (#offset! @docstring 0 3 0 -3))`,
			// Class docstring
			`((class_definition
  body: (block . (expression_statement (string) @rst)))
 (#offset! @rst 0 3 0 -3))`,
			// Module docstring
			`((module . (expression_statement (string) @rst))
 (#offset! @rst 0 3 0 -3))`,
		},
	}
}
