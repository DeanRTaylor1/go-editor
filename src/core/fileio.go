package core

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/deanrtaylor1/go-editor/config"
	"github.com/deanrtaylor1/go-editor/constants"
	"github.com/deanrtaylor1/go-editor/highlighting"
)

func EditorDeleteFile(e *config.Editor, fileName string) error {
	fileName = strings.TrimSuffix(fileName, "\r")

	filePath := filepath.Join(e.RootDirectory, fileName)

	err := os.Remove(filePath)
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	// Remove the buffer if it matches the deleted file
	if e.CurrentBuffer.Name == fileName {
		e.CurrentBuffer = config.NewBuffer() // or however you initialize a new buffer
	}

	// Remove the buffer from the list of buffers
	for i, buffer := range e.Buffers {
		if buffer.Name == fileName {
			e.Buffers = append(e.Buffers[:i], e.Buffers[i+1:]...)
			break
		}
	}

	EditorSetStatusMessage(e, fmt.Sprintf("File %s deleted", fileName))
	return nil
}

func EditorCreateFile(e *config.Editor, fileName string) error {
	fileName = strings.TrimSuffix(fileName, "\r")

	filePath := filepath.Join(e.RootDirectory, fileName)

	// Create the file
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	file.Close()

	EditorSetStatusMessage(e, fmt.Sprintf("File %s created", fileName))
	return nil
}

func EditorRenameFile(e *config.Editor, oldName, newName string) error {
	oldName = strings.TrimSuffix(oldName, "\r")
	newName = strings.TrimSuffix(newName, "\r")
	oldPath := filepath.Join(e.CurrentDirectory, oldName)
	newPath := filepath.Join(e.CurrentDirectory, newName)

	// Check the old file's existence and permissions
	oldFileInfo, err := os.Stat(oldPath)
	if err != nil {
		return fmt.Errorf("failed to stat old file: %w", err)
	}

	// Check if the file is currently open in the editor
	if e.CurrentBuffer.Name == oldName {
		// Save any unsaved changes before renaming
		_, err := EditorSave(e)
		if err != nil {
			config.LogToFile(fmt.Sprintf("%s", err.Error()))
			return fmt.Errorf("failed to save file before renaming: %w", err)
		}
	}

	// Perform the rename
	err = os.Rename(oldPath, newPath)
	if err != nil {
		if os.IsPermission(err) {
			log.Fatal("Permission denied")
		} else {
			config.LogToFile(fmt.Sprintf("%s", err.Error()))
			log.Fatal(err)
		}
	}

	// Check the new file's existence and permissions
	newFileInfo, err := os.Stat(newPath)
	if err != nil {
		return fmt.Errorf("failed to stat new file: %w", err)
	}

	// Optionally, compare old and new FileInfo to ensure they match
	if oldFileInfo.Size() != newFileInfo.Size() || oldFileInfo.Mode() != newFileInfo.Mode() {
		return fmt.Errorf("file info mismatch after rename")
	}

	// Update the current buffer name if it matches the old name
	if e.CurrentBuffer.Name == oldName {
		e.CurrentBuffer.Name = newName
	}

	// Update the name in the list of buffers
	for i, buffer := range e.Buffers {
		if buffer.Name == oldName {
			e.Buffers[i].Name = newName
		}
	}

	// Refresh the file list in your editor here, if applicable

	EditorSetStatusMessage(e, fmt.Sprintf("File renamed from %s to %s", oldName, newName))
	return nil
}

func FileOpen(e *config.Editor, fileName string) error {
	e.EditorMode = constants.EDITOR_MODE_NORMAL
	if !e.FirstRead {
		e.CurrentBuffer = config.NewBuffer()
	}
	e.Cx = 0
	e.Cy = 0
	e.CurrentBuffer.SliceIndex = 0

	file, err := os.Open(fileName)
	if err != nil {
		log.Fatal("Error opening file")
	}
	defer file.Close()
	relativeFileName := strings.TrimPrefix(fileName, e.RootDirectory)

	// If the fileName didn't start with RootDirectory, just use the base name
	if relativeFileName == fileName {
		relativeFileName = filepath.Base(fileName)
	}

	e.FileName = relativeFileName

	highlighting.EditorSelectSyntaxHighlight(e)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		linelen := len(line)
		for linelen > 0 && (line[linelen-1] == '\n' || line[linelen-1] == '\r') {
			linelen--
		}
		row := config.NewRow() // Create a new Row using the NewRow function
		row.Chars = []byte(line[:linelen])
		row.Length = linelen
		row.Idx = len(e.CurrentBuffer.Rows)
		row.Highlighting = make([]byte, row.Length)
		highlighting.Fill(row.Highlighting, constants.HL_NORMAL)
		row.Tabs = make([]byte, row.Length)
		EditorInsertRow(row, row.Idx, e)
		e.CurrentBuffer.NumRows++ // Update NumRows within CurrentBuffer
	}
	highlighting.HighlightFileFromRow(0, e)

	if err := scanner.Err(); err != nil {
		return err
	}
	e.CurrentBuffer.Dirty = 0
	e.FirstRead = false
	e.CurrentBuffer.Name = relativeFileName
	if len(e.Buffers) < 1 {
		e.Buffers = make([]config.Buffer, 0, 15)
	}

	e.LoadNewBuffer()

	EditorSetStatusMessage(e, "HELP: CTRL-S = Save | Ctrl-Q = quit | Ctr-f = find")

	return nil
}

func EditorSave(e *config.Editor) (string, error) {
	if e.CurrentBuffer.Name == "" {
		return "", errors.New("no filename provided")
	}

	startTime := time.Now()
	content := EditorRowsToString(e)

	file, err := os.OpenFile(fmt.Sprintf("%s%s", e.RootDirectory, e.CurrentBuffer.Name), os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	if err := file.Truncate(int64(len(content))); err != nil {
		return "", fmt.Errorf("failed to truncate file: %w", err)
	}

	n, err := file.WriteString(content)
	if err != nil {
		return "", fmt.Errorf("failed to write to file: %w", err)
	}
	if n != len(content) {
		return "", errors.New("unexpected number of bytes written to file")
	}

	elapsedTime := time.Since(startTime) // End timing
	numLines := len(e.CurrentBuffer.Rows)
	numBytes := len(content)
	message := fmt.Sprintf("\"%s\", %dL, %dB, %.3fms: written", e.CurrentBuffer.Name, numLines, numBytes, float64(elapsedTime.Nanoseconds())/1e6)

	e.CurrentBuffer.Dirty = 0

	return message, nil
}

func EditorRowsToString(e *config.Editor) string {
	var buffer strings.Builder
	for _, row := range e.CurrentBuffer.Rows {
		buffer.Write(row.Chars)
		buffer.WriteByte('\n')
	}
	return buffer.String()
}
