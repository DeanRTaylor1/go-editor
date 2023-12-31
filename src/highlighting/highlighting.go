package highlighting

import (
	"path/filepath"
	"strings"
	"unicode"

	"github.com/deanrtaylor1/go-editor/config"
	"github.com/deanrtaylor1/go-editor/constants"
	"github.com/deanrtaylor1/go-editor/utils"
)

func parseToken(i int, chars []byte) (string, int) {
	var token strings.Builder
	length := 0
	for i+length < len(chars) && !isDelimiter(chars[i+length]) {
		token.WriteByte(chars[i+length])
		length++
	}
	return token.String(), length
}

func isDelimiter(c byte) bool {
	return c == ' ' || c == '(' || c == ')' || c == '{' || c == '}' || c == '[' || c == ']' || !unicode.IsLetter(rune(c))
}

func EditorSelectSyntaxHighlight(e *config.Editor) {
	e.CurrentBuffer.BufferSyntax.FileType = "" // Reset to no filetype
	if e.FileName == "" {
		return
	}

	ext := filepath.Ext(e.FileName)
	for _, syntax := range e.CurrentBuffer.BufferSyntax.Syntaxes {
		for _, filematch := range syntax.FileMatch {
			isExt := strings.HasPrefix(filematch, ".")
			if (isExt && ext != "" && ext == filematch) || (!isExt && strings.Contains(e.FileName, filematch)) {
				e.CurrentBuffer.BufferSyntax.FileType = syntax.FileType
				e.CurrentBuffer.BufferSyntax.Flags = syntax.Flags
				e.CurrentBuffer.BufferSyntax.SingleLineCommentStart = syntax.SingleLineCommentStart
				e.CurrentBuffer.BufferSyntax.Keywords = syntax.Syntax
				e.CurrentBuffer.BufferSyntax.MultiLineCommentStart = syntax.MultiLineCommentStart
				e.CurrentBuffer.BufferSyntax.MultiLineCommentEnd = syntax.MultiLineCommentEnd
				return
			}
		}
	}
}

func HighlightFileFromRow(rowStart int, e *config.Editor) {
	for i := rowStart; i < len(e.CurrentBuffer.Rows); i++ {
		SyntaxHighlightStateMachine(&e.CurrentBuffer.Rows[i], e)
	}

	e.CurrentBuffer.NeedsFullHighlight = false
}

func SyntaxHighlightStateMachine(row *config.Row, e *config.Editor) {
	if e.CurrentBuffer.BufferSyntax == nil {
		return
	}
	state := constants.STATE_NORMAL
	scs := e.CurrentBuffer.BufferSyntax.SingleLineCommentStart
	mcs := e.CurrentBuffer.BufferSyntax.MultiLineCommentStart
	mce := e.CurrentBuffer.BufferSyntax.MultiLineCommentEnd
	scsLen, mcsLen, mceLen := len(scs), len(mcs), len(mce)
	inString := byte(0)
	if row.Idx > 0 {
		if e.CurrentBuffer.Rows[row.Idx-1].HlOpenComment {
			row.HlOpenComment = true
			state = constants.STATE_MLCOMMENT
		} else {
			row.HlOpenComment = false
		}
	}

	i := 0
	for i < row.Length {
		c := row.Chars[i]

		switch state {
		case constants.STATE_NORMAL:
			row.Highlighting[i] = constants.HL_NORMAL

			if scsLen > 0 && i+scsLen < row.Length && string(row.Chars[i:i+scsLen]) == scs {
				state = constants.STATE_SLCOMMENT
				for j := i; j < i+scsLen; j++ {
					row.Highlighting[j] = constants.HL_COMMENT
				}
				i += scsLen - 1
			} else if mcsLen > 0 && i+mcsLen <= row.Length && string(row.Chars[i:i+mcsLen]) == mcs {
				row.HlOpenComment = true
				state = constants.STATE_MLCOMMENT
				for j := i; j < i+mcsLen; j++ {
					row.Highlighting[j] = constants.HL_MLCOMMENT
				}
				i += mcsLen - 1
				if !e.CurrentBuffer.NeedsFullHighlight {
					e.CurrentBuffer.NeedsFullHighlight = true
					HighlightFileFromRow(row.Idx, e)
					return
				}
			} else if c == '"' || c == '\'' {
				inString = c
				state = constants.STATE_STRING
				row.Highlighting[i] = constants.HL_STRING
				i++
			} else if utils.IsDigit(c) {
				isPrevCharValid := i == 0 || isDelimiter(row.Chars[i-1])
				isNextCharValid := i+1 >= row.Length || isDelimiter(row.Chars[i+1])

				if isPrevCharValid && isNextCharValid {
					state = constants.STATE_NUMBER
					row.Highlighting[i] = constants.HL_NUMBER
				} else {
					state = constants.STATE_NORMAL
				}
				i++

			} else if c != ' ' {
				token, length := parseToken(i, row.Chars)
				isPrevCharValid := i == 0 || isDelimiter(row.Chars[i-1])
				isNextCharValid := i+length >= row.Length || isDelimiter(row.Chars[i+length])
				if category, exists := e.CurrentBuffer.BufferSyntax.Keywords[token]; exists && isPrevCharValid && isNextCharValid {
					for j := 0; j < length; j++ {
						row.Highlighting[i+j] = category
					}
					i += length
				} else {
					i++
				}
			} else {
				i++
			}
			if i >= row.Length {
				break // Exit the loop if we've reached the end of the line
			}

		case constants.STATE_MLCOMMENT:
			row.Highlighting[i] = constants.HL_MLCOMMENT
			if i+mceLen <= row.Length && string(row.Chars[i:i+mceLen]) == mce {
				for j := i; j < i+mceLen; j++ {
					row.Highlighting[j] = constants.HL_MLCOMMENT
				}
				row.HlOpenComment = false
				i += mceLen
				state = constants.STATE_NORMAL
				if !e.CurrentBuffer.NeedsFullHighlight {
					e.CurrentBuffer.NeedsFullHighlight = true
					HighlightFileFromRow(row.Idx, e)
					return
				}
			} else {
				i++
			}
		case constants.STATE_STRING:
			for i < row.Length && row.Chars[i] != inString {
				if row.Chars[i] == '\\' && i+1 < row.Length {
					row.Highlighting[i] = constants.HL_STRING
					i++
				}
				row.Highlighting[i] = constants.HL_STRING
				i++
			}
			if i < row.Length && row.Chars[i] == inString { // Handle closing quote
				row.Highlighting[i] = constants.HL_STRING
				i++
			}
			state = constants.STATE_NORMAL
			inString = byte(0)

		case constants.STATE_NUMBER:
			isPrevCharValid := i == 0 || isDelimiter(row.Chars[i-1]) || unicode.IsDigit(rune(row.Chars[i-1]))
			isNextCharValid := i+1 >= row.Length || isDelimiter(row.Chars[i+1]) || unicode.IsDigit(rune(row.Chars[i+1]))

			if unicode.IsDigit(rune(c)) || (c == '.' && row.Highlighting[i-1] == constants.HL_NUMBER && isNextCharValid && isPrevCharValid) {
				row.Highlighting[i] = constants.HL_NUMBER
				i++
			} else {
				state = constants.STATE_NORMAL // Transition back to normal state if non-digit found
			}
		case constants.STATE_SLCOMMENT:
			for ; i < row.Length; i++ {
				row.Highlighting[i] = constants.HL_COMMENT
			}
			return

		}
	}

	if !e.CurrentBuffer.NeedsFullHighlight && row.HlOpenComment && i >= row.Length {
		e.CurrentBuffer.NeedsFullHighlight = true
		HighlightFileFromRow(0, e)
	}
}

func isSeparator(c rune) bool {
	return unicode.IsSpace(c) || c == '\x00' || strings.ContainsRune(",.()+-/*=~%<>[];", c)
}

func ResetRowHighlights(offset int, e *config.Editor) {
	currentRow := &e.CurrentBuffer.Rows[e.Cy+offset]

	currentRow.Highlighting = make([]byte, currentRow.Length)
	Fill(currentRow.Highlighting, constants.HL_NORMAL)
}

func Fill(slice []byte, value byte) {
	for i := range slice {
		slice[i] = value
	}
}

func EditorSyntaxToColor(highlight byte) byte {
	switch highlight {
	case constants.HL_CONTROL_FLOW:
		return 35 // Magenta
	case constants.HL_VARIABLE:
		return 34 // Blue
	case constants.HL_CONSTANT:
		return 32 // Green
	case constants.HL_TYPE:
		return 33 // Yellow
	case constants.HL_FUNCTION:
		return 36 // Cyan
	case constants.HL_PREPROCESSOR:
		return 90 // Bright Black (Gray)
	case constants.HL_STORAGE_CLASS:
		return 94 // Bright Blue
	case constants.HL_OPERATOR:
		return 37 // White
	case constants.HL_MLCOMMENT, constants.HL_COMMENT:
		return 90 // Cyan
	case constants.HL_STRING:
		return 92 // Bright Red
	case constants.HL_NUMBER:
		return 31 // Red
	case constants.HL_BOOLEAN:
		return 33 // Yellow
	case constants.HL_KEYWORD:
		return 35 // Magenta
	case constants.HL_BUILTIN:
		return 31 // Blue
	case constants.HL_ANNOTATION:
		return 30 // Black
	case constants.HL_EXCEPTION:
		return 91 // Bright Red
	case constants.HL_MODULE:
		return 34 // Blue
	case constants.HL_DEBUG:
		return 90 // Bright Black (Gray)
	case constants.HL_TEST:
		return 32 // Green
	case constants.HL_DOCUMENTATION:
		return 93 // Bright Yellow
	case constants.HL_MATCH:
		return 93
	default:
		return 37 // White
	}
}
