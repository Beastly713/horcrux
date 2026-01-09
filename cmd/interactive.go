package cmd

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg" // Support JPEG decoding
	_ "image/png"  // Support PNG decoding
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Beastly713/horcrux/pkg/format"
	"github.com/Beastly713/horcrux/pkg/pipeline"
	"github.com/Beastly713/horcrux/pkg/shamir"
	"github.com/Beastly713/horcrux/pkg/stego"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

// Styles
var (
	focusedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	cursorStyle  = focusedStyle.Copy()
	checkedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42")) // Green
	docStyle     = lipgloss.NewStyle().Margin(1, 2)
)

type fileItem struct {
	path     string
	name     string
	isDir    bool
	selected bool
}

type model struct {
	path       string
	files      []fileItem
	cursor     int
	status     string
	textInput  textinput.Model // For naming output if needed, or simple status
	quitting   bool
	processing bool
}

func initialModel() model {
	cwd, _ := os.Getwd()
	m := model{
		path:   cwd,
		status: "Navigate: ↑/↓ | Enter: Open Dir | Space: Select | 'b': Bind Selected",
	}
	m.loadFiles()
	return m
}

func (m *model) loadFiles() {
	entries, err := os.ReadDir(m.path)
	if err != nil {
		m.status = "Error reading directory"
		return
	}

	m.files = []fileItem{}
	// Parent directory
	m.files = append(m.files, fileItem{name: "..", isDir: true, path: filepath.Dir(m.path)})

	for _, e := range entries {
		name := e.Name()
		// Simple filter for relevance
		isRel := e.IsDir() || strings.HasSuffix(name, ".horcrux") || strings.HasSuffix(name, ".png")
		if isRel {
			m.files = append(m.files, fileItem{
				name:  name,
				isDir: e.IsDir(),
				path:  filepath.Join(m.path, name),
			})
		}
	}
	m.cursor = 0
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			if m.cursor < len(m.files)-1 {
				m.cursor++
			}

		case "enter":
			selected := m.files[m.cursor]
			if selected.isDir {
				m.path = selected.path
				m.loadFiles()
			}

		case " ":
			if !m.files[m.cursor].isDir {
				m.files[m.cursor].selected = !m.files[m.cursor].selected
			}

		case "b":
			// Trigger Bind logic
			return m, m.bindSelected()
		}

	case statusMsg:
		m.status = string(msg)
		if strings.HasPrefix(m.status, "Success") {
			// Clear selections on success
			for i := range m.files {
				m.files[i].selected = false
			}
		}
	}

	return m, nil
}

type statusMsg string

func (m model) bindSelected() tea.Cmd {
	return func() tea.Msg {
		var selectedPaths []string
		for _, f := range m.files {
			if f.selected {
				selectedPaths = append(selectedPaths, f.path)
			}
		}

		if len(selectedPaths) == 0 {
			return statusMsg("No files selected!")
		}

		if err := runInteractiveBind(selectedPaths); err != nil {
			return statusMsg(fmt.Sprintf("Error: %v", err))
		}

		return statusMsg("Success! File resurrected in current directory.")
	}
}

func (m model) View() string {
	if m.quitting {
		return "Bye!\n"
	}

	s := fmt.Sprintf("Directory: %s\n\n", m.path)

	for i, file := range m.files {
		cursor := " " // no cursor
		if m.cursor == i {
			cursor = ">"
			s += cursorStyle.Render(cursor)
		} else {
			s += cursor
		}

		checked := " "
		if file.selected {
			checked = "x"
		}

		line := ""
		if file.isDir {
			line = fmt.Sprintf("[DIR] %s", file.name)
		} else {
			line = fmt.Sprintf("[%s] %s", checked, file.name)
		}

		if file.selected {
			line = checkedStyle.Render(line)
		}

		s += " " + line + "\n"
	}

	s += fmt.Sprintf("\n%s\n", m.status)
	return docStyle.Render(s)
}

// runInteractiveBind is a simplified version of the core bind logic
// adapted for the TUI to run on specific selected files.
func runInteractiveBind(paths []string) error {
	// Group files by ID
	type loadedHorcrux struct {
		Header *format.Header
		Body   []byte // Load full body for simplicity in TUI
	}
	
	// We only process one group for simplicity in this interactive mode
	// or we take the first valid group we find from selection.
	var horcruxes []*loadedHorcrux
	var refHeader *format.Header

	for _, path := range paths {
		// 1. Open & Handle Stego/Normal
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		var reader io.Reader = file
		
		// Check for PNG stego
		if strings.HasSuffix(strings.ToLower(path), ".png") {
			img, _, err := image.Decode(file)
			if err != nil {
				return fmt.Errorf("failed to decode image %s: %w", filepath.Base(path), err)
			}
			hiddenData, err := stego.Extract(img)
			if err != nil {
				return fmt.Errorf("stego extraction failed for %s: %w", filepath.Base(path), err)
			}
			reader = bytes.NewReader(hiddenData)
		}

		// 2. Parse Header
		hReader, err := format.NewReader(reader)
		if err != nil {
			return fmt.Errorf("invalid header in %s: %w", filepath.Base(path), err)
		}

		// Read Body
		body, err := io.ReadAll(hReader.Body)
		if err != nil {
			return err
		}

		if refHeader == nil {
			refHeader = hReader.Header
		} else {
			// Basic validation that they belong to same file
			if hReader.Header.OriginalFilename != refHeader.OriginalFilename {
				return fmt.Errorf("selection contains mixed files: %s vs %s", refHeader.OriginalFilename, hReader.Header.OriginalFilename)
			}
		}

		horcruxes = append(horcruxes, &loadedHorcrux{
			Header: hReader.Header,
			Body:   body,
		})
	}

	if len(horcruxes) < refHeader.Threshold {
		return fmt.Errorf("not enough shards. Need %d, selected %d", refHeader.Threshold, len(horcruxes))
	}

	// 3. Reconstruct
	keyFragments := make([][]byte, len(horcruxes))
	shardMap := make(map[int][]byte)

	for i, h := range horcruxes {
		keyFragments[i] = h.Header.KeyFragment
		
		// CRITICAL FIX: Convert 1-based Header Index to 0-based RS Index
		shardMap[h.Header.Index-1] = h.Body
	}

	key, err := shamir.Combine(keyFragments)
	if err != nil {
		return fmt.Errorf("key reconstruction failed: %w", err)
	}

	plainText, err := pipeline.JoinPipeline(shardMap, key, refHeader.Total, refHeader.Threshold)
	if err != nil {
		return fmt.Errorf("decryption pipeline failed: %w", err)
	}

	// 4. Save
	// We save to the current working directory of the TUI user
	cwd, _ := os.Getwd()
	outPath := filepath.Join(cwd, refHeader.OriginalFilename)
	
	if err := os.WriteFile(outPath, plainText, 0644); err != nil {
		return err
	}

	return nil
}

// Cobra command setup
var interactiveCmd = &cobra.Command{
	Use:   "interactive",
	Short: "Interactive terminal UI for binding horcruxes",
	RunE: func(cmd *cobra.Command, args []string) error {
		p := tea.NewProgram(initialModel())
		if _, err := p.Run(); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(interactiveCmd)
}