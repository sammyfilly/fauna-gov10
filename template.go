package fauna

import (
	"fmt"
	"regexp"
)

type templateCategory string

const (
	templateVariable templateCategory = "variable"
	templateLiteral  templateCategory = "literal"
)

type templatePart struct {
	Text     string
	Category templateCategory
}

type template struct {
	text string
	re   *regexp.Regexp
}

func newTemplate(text string) *template {
	return &template{
		text: text,
		re:   regexp.MustCompile(`\$(?:(?P<escaped>\$)|{(?P<braced>[_a-zA-Z0-9]*)}|(?P<invalid>))`),
	}
}

// Parse parses Text and returns a slice of template parts.
func (t *template) Parse() ([]templatePart, error) {
	escapedIndex := t.re.SubexpIndex("escaped")
	bracedIndex := t.re.SubexpIndex("braced")
	invalidIndex := t.re.SubexpIndex("invalid")

	end := len(t.text)
	currentPosition := 0

	matches := t.re.FindAllStringSubmatch(t.text, -1)
	matchIndexes := t.re.FindAllStringSubmatchIndex(t.text, -1)
	parts := make([]templatePart, 0)

	for i, m := range matches {
		matchIndex := matchIndexes[i]
		invalidStartPos := matchIndex[invalidIndex*2]
		if invalidStartPos >= 0 {
			// TODO: Improve with line/column num
			return nil, fmt.Errorf("invalid placeholder in template: position %d", invalidStartPos)
		}

		matchStartPos := matchIndex[0]
		matchEndPos := matchIndex[1]
		escaped := m[escapedIndex]
		variable := m[bracedIndex]

		if currentPosition < matchStartPos {
			parts = append(parts, templatePart{
				Text:     t.text[currentPosition:matchStartPos] + escaped,
				Category: templateLiteral,
			})
		}

		if len(variable) > 0 {
			parts = append(parts, templatePart{
				Text:     variable,
				Category: templateVariable,
			})
		}

		currentPosition = matchEndPos
	}

	if currentPosition < end {
		parts = append(parts, templatePart{Text: t.text[currentPosition:], Category: templateLiteral})
	}

	return parts, nil
}
