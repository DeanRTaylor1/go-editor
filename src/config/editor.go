package config

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/deanrtaylor1/go-editor/constants"
	"github.com/deanrtaylor1/go-editor/fuzzy"
	"github.com/deanrtaylor1/go-editor/utils"
)

type Point struct {
	Row int
	Col int
}

const (
	MODAL_TYPE_FUZZY = iota
	MODAL_TYPE_GENERIC
)

type Modal struct {
	Type            int
	ModalInput      []byte
	CursorPosition  int
	Data            interface{}
	Results         interface{}
	ItemIndex       int
	DataRowOffset   int
	SearchColOffset int
	ModalDrawn      bool
}

type Editor struct {
	EditorMode             int
	Cx                     int
	Cy                     int
	SliceIndex             int
	LineNumberWidth        int
	ScreenRows             int
	ScreenCols             int
	TerminalState          *term.State
	CurrentBuffer          *Buffer
	Buffers                []Buffer
	RowOff                 int
	ColOff                 int
	FileName               string
	StatusMsg              string
	StatusMsgTime          time.Time
	QuitTimes              int
	Reader                 *bufio.Reader
	FirstRead              bool
	UndoHistory            int
	CurrentDirectory       string
	RootDirectory          string
	FileBrowserItems       []FileBrowserItem
	FileBrowserActionState FileBrowserActionState
	FileBrowserIntroLength int
	MotionBuffer           []rune
	MotionMap              map[string]func()
	Yank                   Yank
	ModalOpen              bool
	Modal                  Modal
}

func (e *Editor) ClearModalInput() {
	e.Modal.ModalInput = []byte{}
}

func NewEditor() *Editor {
	return &Editor{
		EditorMode:       constants.EDITOR_MODE_NORMAL,
		Cx:               0,
		Cy:               0,
		LineNumberWidth:  5,
		ScreenRows:       0,
		ScreenCols:       0,
		TerminalState:    nil,
		CurrentBuffer:    NewBuffer(),
		RowOff:           0,
		ColOff:           0,
		FileName:         "[Not Selected]",
		StatusMsg:        "",
		StatusMsgTime:    time.Time{},
		QuitTimes:        constants.QUIT_TIMES,
		Reader:           bufio.NewReader(os.Stdin),
		FirstRead:        true,
		UndoHistory:      30,
		FileBrowserItems: []FileBrowserItem{},
		CurrentDirectory: "",
		MotionBuffer:     []rune{},
		ModalOpen:        false,
	}
}

func (m *Modal) ResetToFirstItem() {
	m.ItemIndex = 0
	m.DataRowOffset = 0
}

func (m *Modal) String(i int) string {
	switch m.Type {
	case MODAL_TYPE_FUZZY:
		data, ok := m.Data.(fuzzy.Matches)
		if ok && i < len(data) {
			return data[i].Str
		}
	default:
		data, ok := m.Data.([]string)
		if ok && i < len(data) {
			return data[i]
		}
	}
	return ""
}

func (m *Modal) Len() int {
	switch m.Type {
	case MODAL_TYPE_FUZZY:
		data, ok := m.Data.(fuzzy.Matches)
		if ok {
			return len(data)
		}
	default:
		data, ok := m.Data.([]string)
		if ok {
			return len(data)
		}
	}
	return 0
}

func ListFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			// Remove the dir prefix from the path
			relativePath := strings.TrimPrefix(path, dir)
			if strings.HasPrefix(relativePath, string(os.PathSeparator)) {
				relativePath = strings.TrimPrefix(relativePath, string(os.PathSeparator))
			}
			files = append(files, relativePath)
		}
		return nil
	})
	return files, err
}

func InitModal(modalType int) Modal {
	switch modalType {
	case MODAL_TYPE_FUZZY:
		return Modal{
			Type:            modalType,
			ModalInput:      []byte{},
			CursorPosition:  0,
			Data:            fuzzy.Matches{},
			ItemIndex:       0,
			DataRowOffset:   0,
			SearchColOffset: 0,
			Results:         NewResults(),
		}
	default:
		return Modal{
			Type:            modalType,
			ModalInput:      []byte{},
			CursorPosition:  0,
			Data:            []string{},
			ItemIndex:       0,
			DataRowOffset:   0,
			SearchColOffset: 0,
			Results:         []string{},
		}
	}
}

func NewResults() fuzzy.Matches {
	return fuzzy.Matches{}
}

func (e *Editor) DeleteSelection() {
	startPoint, endPoint := e.GetNormalizedSelection()

	// Case 1: Start and end points are on the same row
	if startPoint.Row == endPoint.Row {
		endPointCursor := endPoint.Col - e.LineNumberWidth + 1
		row := &e.CurrentBuffer.Rows[startPoint.Row]
		if endPointCursor > row.Length {
			endPointCursor = row.Length
		}
		row.Chars = append(row.Chars[:startPoint.Col-e.LineNumberWidth], row.Chars[endPointCursor:]...)
		row.Length = len(row.Chars)
	} else {
		// Case 2: Start and end points are on different rows

		// Update the start row
		startRow := &e.CurrentBuffer.Rows[startPoint.Row]
		startRow.Chars = startRow.Chars[:startPoint.Col-e.LineNumberWidth]
		startRow.Length = len(startRow.Chars)

		// Update the end row
		endRow := &e.CurrentBuffer.Rows[endPoint.Row]
		endRow.Chars = endRow.Chars[endPoint.Col-e.LineNumberWidth+1:]
		endRow.Length = len(endRow.Chars)

		// Remove the rows between start and end
		e.CurrentBuffer.RemoveRowsFromIndex(startPoint.Row+1, endPoint.Row-startPoint.Row-1)

		// Merge the start and end rows
		startRow.Chars = append(startRow.Chars, endRow.Chars...)
		startRow.Length = len(startRow.Chars)

		// Remove the end row (which is now at startPoint.Row + 1 after removing the middle rows)
		e.CurrentBuffer.RemoveRowAtIndex(startPoint.Row + 1)
	}
	e.Cx = e.LineNumberWidth
	e.CurrentBuffer.SliceIndex = 0
}

func (e *Editor) YankSelection() {
	startPoint, endPoint := e.GetNormalizedSelection()
	partialBuffer := Buffer{
		Rows:    []Row{},
		NumRows: 0,
	}

	for i := startPoint.Row; i <= endPoint.Row; i++ {
		row := e.CurrentBuffer.Rows[i]
		newRow := Row{}
		if i == startPoint.Row && i == endPoint.Row {
			newRow.Chars = row.Chars[startPoint.Col-e.LineNumberWidth : endPoint.Col-e.LineNumberWidth+1]
		} else if i == startPoint.Row {
			newRow.Chars = row.Chars[startPoint.Col-e.LineNumberWidth:]
		} else if i == endPoint.Row {
			newRow.Chars = row.Chars[:endPoint.Col-e.LineNumberWidth]
		} else {
			newRow.Chars = row.Chars
		}
		newRow.Chars = bytes.Trim(newRow.Chars, "\x00")
		partialBuffer.Rows = append(partialBuffer.Rows, newRow)
		partialBuffer.NumRows++
	}

	e.Yank.PartialBuffer = partialBuffer
	if partialBuffer.NumRows == 1 {
		e.Yank.Type = CharWise
	} else {
		e.Yank.Type = LineWise
	}
}

func (e *Editor) IsWithinSelection(fileRow, col int, startPoint, endPoint Point) bool {
	withinSelection := false
	if fileRow == startPoint.Row && fileRow == endPoint.Row {
		withinSelection = (e.ColOff+col >= startPoint.Col-e.LineNumberWidth && col <= endPoint.Col-e.LineNumberWidth)
	} else if fileRow == startPoint.Row {
		withinSelection = (e.ColOff+col >= startPoint.Col-e.LineNumberWidth)
	} else if fileRow == endPoint.Row {
		withinSelection = (e.ColOff+col <= endPoint.Col-e.LineNumberWidth)
	} else if fileRow > startPoint.Row && fileRow < endPoint.Row {
		withinSelection = true
	}
	return withinSelection
}

func (e *Editor) ClearYank() {
	e.Yank = Yank{PartialBuffer: *NewBuffer(), Type: EMPTY_YANK}
}

// Get the normalized selection start and end points
func (e *Editor) GetNormalizedSelection() (Point, Point) {
	start := e.CurrentBuffer.SelectionStart
	end := e.CurrentBuffer.SelectionEnd

	if start.Row > end.Row || (start.Row == end.Row && start.Col > end.Col) {
		return end, start
	}
	return start, end
}

func (e *Editor) ClearSelection() {
	neutralPoint := Point{Col: -1, Row: -1}
	e.CurrentBuffer.SelectionStart = neutralPoint
	e.CurrentBuffer.SelectionEnd = neutralPoint
}

func (e *Editor) MoveSelection() {
	e.CurrentBuffer.SelectionEnd = Point{Col: e.Cx, Row: e.Cy}
}

func (e *Editor) HighlightSelection() {
	e.CurrentBuffer.SelectionStart = Point{Col: e.Cx, Row: e.Cy}
	e.CurrentBuffer.SelectionEnd = Point{Col: e.Cx, Row: e.Cy}
}

func (e *Editor) HighlightLine() {
	e.CurrentBuffer.SelectionStart = Point{Col: e.LineNumberWidth, Row: e.Cy}
	e.CurrentBuffer.SelectionEnd = Point{Col: e.GetCurrentRow().Length + e.LineNumberWidth, Row: e.Cy}
}

func (e *Editor) YankChars() {
}

func (e *Editor) YankLine() {
}

func (e *Editor) YankBlock() {
}

func (e *Editor) ResetCursorCoords() {
	e.Cx = 0
	e.Cy = 0
	e.ColOff = 0
	e.RowOff = 0
}

func (e *Editor) CacheCursorCoords() {
	e.CurrentBuffer.StoredCx = e.Cx
	e.CurrentBuffer.StoredCy = e.Cy
	e.CurrentBuffer.StoredOffsetY = e.RowOff
	e.CurrentBuffer.StoredOffsetX = e.ColOff
}

func (e *Editor) ClearMotionBuffer() {
	e.MotionBuffer = []rune{}
}

func (e *Editor) ExecuteMotion(motion string) bool {
	if action, exists := e.MotionMap[motion]; exists {
		action()
		return true
	}
	return false
}

func (e *Editor) InstructionsLines() []string {
	return []string{
		"==========================================================",
		fmt.Sprintf("\x1b[36mGVim\x1b[39m Version: %s", constants.VERSION),
		"Gvim File Directory Preview",
		fmt.Sprintf("RootDir: %s", e.RootDirectory),
		"sorted by: name",
		"File Management: \x1b[38;5;1m%: Create R: Rename, D: Delete\x1b[39m",
		"Navigation:\x1b[38;5;1m<Up>/<k> to go up, <Down>/<j> to go down, <Enter> to select\x1b[39m",
		"==========================================================",
	}
}

func (e *Editor) IsDir() bool {
	return e.CurrentSelectedFile().Name == ".." || e.CurrentSelectedFile().Type == "directory"
}

func (e *Editor) CurrentSelectedFile() *FileBrowserItem {
	return &e.FileBrowserItems[e.Cy-len(e.InstructionsLines())]
}

// ReplaceBuffer replaces the buffer that matches the name of the current buffer with the current buffer's state.
func (e *Editor) ReplaceBuffer() {
	for i, bufferItem := range e.Buffers {
		if bufferItem.Name == e.CurrentBuffer.Name {
			// Replace the item with the current state
			e.Buffers[i] = *e.CurrentBuffer
			break
		}
	}
}

// ReloadBuffer reloads the buffer that matches the name of the current buffer.
// It returns true if a matching buffer is found, false otherwise.
func (e *Editor) ReloadBuffer(path string) bool {
	for _, bufferItem := range e.Buffers {
		if e.RootDirectory+bufferItem.Name == path {
			// Load the old buffer into CurrentBuffer
			e.CurrentBuffer = &bufferItem
			e.Cx = bufferItem.StoredCx
			e.Cy = bufferItem.StoredCy
			e.ColOff = bufferItem.StoredOffsetX
			e.RowOff = bufferItem.StoredOffsetY
			return true
		}
	}
	return false
}

func (e *Editor) LoadNewBuffer() {
	newBuffer := *e.CurrentBuffer
	newBuffer.Idx = len(e.Buffers)
	e.Buffers = append(e.Buffers, newBuffer)
}

func (e *Editor) RemoveBuffer(name string) {
	idx := -1
	for i, v := range e.Buffers {
		if v.Name == name {
			idx = i
			break
		}
	}

	if idx != -1 {
		e.Buffers = append(e.Buffers[:idx], e.Buffers[idx+1:]...)
	}
}

func (c *Editor) ClearRedoStack() {
	c.CurrentBuffer.RedoStack = []EditorAction{}
}

func (c *Editor) GetAdjustedCx() int {
	adjustedCx := utils.Max(0, c.Cx-c.LineNumberWidth)
	adjustedCx = utils.Min(adjustedCx, len(c.CurrentBuffer.Rows[c.Cy].Chars))
	return adjustedCx
}

func (c *Editor) GetCurrentRow() *Row {
	return &c.CurrentBuffer.Rows[c.Cy]
}

func (e *Editor) SpecialRefreshCase() bool {
	return e.Cx >= e.ScreenCols-e.LineNumberWidth || e.Cy >= e.ScreenRows || e.Cx-e.LineNumberWidth < e.ColOff || e.Cy-e.RowOff < 0 || e.Cx-e.ColOff == 5
}

func (e *Editor) IsBrowsingFiles() bool {
	return e.EditorMode == constants.EDITOR_MODE_FILE_BROWSER
}

func (e *Editor) SetMode(mode int) {
	e.EditorMode = mode
}

func (e *Editor) MoveCursorLeft() {
	e.Cx--
	if e.EditorMode != constants.EDITOR_MODE_FILE_BROWSER {
		e.CurrentBuffer.SliceIndex--
	}
}

func (e *Editor) MoveCursorRight() {
	if e.EditorMode != constants.EDITOR_MODE_FILE_BROWSER {
		e.CurrentBuffer.SliceIndex++
	}
	e.Cx++
}

func (e *Editor) MoveCursorUp() {
	e.Cy--
}

func (e *Editor) MoveCursorDown() {
	e.Cy++
}
