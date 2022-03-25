package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"charm_runner/circular_buffer"
	"charm_runner/debug"
	"charm_runner/process"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wrap"
)

// bubbletea
const useHighPerformanceRenderer = false

type configuration struct {
	Name     string
	Commands []process.ProgramState
}

type model struct {
	ready           bool
	waitingOnConfig bool
	showingHelp     bool
	programs        []process.ProgramState
	message         string
	outViewport     viewport.Model
	err             error
}

type errMsg struct{ err error }

// For messages that contain errors it's often handy to also implement the
// error interface on the message.
func (e errMsg) Error() string { return e.err.Error() }

var (
	globalError error

	// red background
	errorStyle = lipgloss.NewStyle().
		Background(lipgloss.Color("9"))

	// light blue background
	highlightStyle = lipgloss.NewStyle().
		Background(lipgloss.Color("14"))
)

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func loadConfigFile() tea.Msg {
	file, err := ioutil.ReadFile("config.json")

	if err != nil {
		debug.DumpStringToDebugListener(fmt.Sprintln("... err on reading config.json is", err))
		return errMsg{err}
	}

	var data configuration
	err = json.Unmarshal(file, &data)

	if err != nil {
		debug.DumpStringToDebugListener(fmt.Sprintln("... err on unmarshalling json is", err))
		return errMsg{err}
	}

	return data
}

func initialModel() model {
	return model{
		waitingOnConfig: true,
		programs:        make([]process.ProgramState, 0),
	}
}

func (m model) helpView() string {
	s := " Program runner help: "
	line1 := strings.Repeat("─", 2)
	line2 := strings.Repeat("─", max(0, m.outViewport.Width-lipgloss.Width(s)-lipgloss.Width(line1)))
	s = lipgloss.JoinHorizontal(lipgloss.Center, line1, s, line2)

	s += "\n\nPress:\n  h to enter/exit help\n  q to quit.\n  r to reload the configuration.\n\n"

	s += "Up/down arrow keys, PgUp/PgDn scroll the output window.\n"

	return s
}

func (m model) titleView() string {
	title1 := " Program runner "
	line1 := strings.Repeat("─", 2)
	title2 := " (h)elp "
	line2 := strings.Repeat("─", max(0, m.outViewport.Width-lipgloss.Width(title1)-lipgloss.Width(title2)-2*lipgloss.Width(line1)))
	return lipgloss.JoinHorizontal(lipgloss.Center, line1, title1, line2, title2, line1)
}

func (m model) outputTitleView() string {
	title := " Output "
	line1 := strings.Repeat("─", 2)
	line2 := strings.Repeat("─", max(0, m.outViewport.Width-lipgloss.Width(title)-lipgloss.Width(line1)))
	return lipgloss.JoinHorizontal(lipgloss.Center, line1, title, line2)
}

func (m model) headerView() string {
	debug.DumpStringToDebugListener(fmt.Sprintf("headerView sees model with %d programs.", len(m.programs)))
	s := "\n"
	s += "Start/Stop │ View output │ Running? │ Program\n"
	s += "───────────┼─────────────┼──────────┼─────────\n"

	for index := range m.programs {
		runningState := " "
		showError := false
		if m.programs[index].ProgramRunning {
			runningState = "Y"
		} else if m.programs[index].ProgramRan && !m.programs[index].ProgramSuccess {
			runningState = "Error!"
			showError = true
		} else if m.programs[index].ProgramRan {
			runningState = "Done"
		}

		command := m.programs[index].ProgramCommand
		if strings.Index(command, "/") == 0 {
			args := strings.Split(command, " ")
			programParts := strings.Split(args[0], "/")
			command = programParts[len(programParts)-1] + " " + strings.Join(args[1:], " ")
		}

		runningStateOut := fmt.Sprintf("  %-8s", runningState)
		if showError {
			runningStateOut = errorStyle.Render(runningStateOut)
		}

		viewOutputCharOut := fmt.Sprintf("  %-8s", m.programs[index].ViewOutputChar)
		if m.programs[index].ProgramRan && m.programs[index].ShowingOutputNow {
			viewOutputCharOut = highlightStyle.Render(viewOutputCharOut)
		}

		s += fmt.Sprintf(" %-10s│  %-10s │%s│ %s\n",
			m.programs[index].StartStopChar, viewOutputCharOut,
			runningStateOut, command)
	}

	// Send the UI for rendering
	return s
}

func (m model) footerView() string {
	info := fmt.Sprintf("%4.f%%", m.outViewport.ScrollPercent()*100)
	line := strings.Repeat("─", max(0, m.outViewport.Width-lipgloss.Width(info)))
	return lipgloss.JoinHorizontal(lipgloss.Center, line, info)
}

func (m model) Init() tea.Cmd {
	return loadConfigFile
}

var lastHeight int
var lastWidth int

func updateLayout(m model) model {
	headerHeight := lipgloss.Height(m.titleView() + m.headerView() + m.outputTitleView())
	footerHeight := lipgloss.Height(m.footerView())
	verticalMarginHeight := headerHeight + footerHeight
	debug.DumpStringToDebugListener(fmt.Sprintf("updateLayout headerHeight is %d", headerHeight))
	viewportHeight := lastHeight - verticalMarginHeight - 3

	if !m.ready {
		// Since this program is using the full size of the viewport we
		// need to wait until we've received the window dimensions before
		// we can initialize the viewport. The initial dimensions come in
		// quickly, though asynchronously, which is why we wait for them
		// here.
		m.outViewport = viewport.New(lastWidth, viewportHeight)
		m.outViewport.HighPerformanceRendering = useHighPerformanceRenderer
		m.outViewport.SetContent("")
		m.ready = true

		// Render the viewport one line below the header.
		m.outViewport.YPosition = headerHeight + 1
	} else {
		m.outViewport.Width = lastWidth
		m.outViewport.Height = viewportHeight
	}

	return m
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {

	case errMsg:
		debug.DumpStringToDebugListener(fmt.Sprintln("... update got an errMsg with error", msg))
		globalError = msg
		return m, tea.Quit

	case configuration:
		m.waitingOnConfig = false
		m.programs = msg.Commands
		startStop := '1'
		view := 'a'
		for index := range m.programs {
			m.programs[index].ProgramIndex = index
			m.programs[index].StartStopChar = string(startStop)
			m.programs[index].ViewOutputChar = string(view)
			m.programs[index].ProgramStdOut = circular_buffer.MakeCircularBuffer(100)
			m.programs[index].ProgramStdErr = circular_buffer.MakeCircularBuffer(100)
			m.programs[index].ProgramFinalMessage = "Program not run yet."
			startStop += 1
			view += 1
			if view == 'q' {
				view = 's'
			}
		}

		debug.DumpStringToDebugListener(fmt.Sprintf("configuration got with lastHeight %d, lastWidth %d", lastHeight, lastWidth))
		m = updateLayout(m)

		return m, nil

	case tea.WindowSizeMsg:
		debug.DumpStringToDebugListener(fmt.Sprintf("tea.WindowSizeMsg got with msg %v", msg))
		lastHeight = msg.Height
		lastWidth = msg.Width
		m = updateLayout(m)

		if useHighPerformanceRenderer {
			// Render (or re-render) the whole viewport. Necessary both to
			// initialize the viewport and when the window is resized.
			//
			// This is needed for high-performance rendering only.
			cmds = append(cmds, viewport.Sync(m.outViewport))
		}

	case process.ProgramFinishedMessage:
		m.message = msg.ProgramOutput
		m.programs[msg.ProgramIndex].ProgramRan = true
		m.programs[msg.ProgramIndex].ProgramRunning = false
		m.programs[msg.ProgramIndex].ProgramSuccess = msg.ProgramSuccess
		m.programs[msg.ProgramIndex].ProgramFinalMessage = msg.ProgramOutput
		return m, nil

	// Is it a key press?
	case tea.KeyMsg:
		// Cool, what was the actual key pressed?
		ch := msg.String()
		switch ch {

		// Exit the program.
		case "ctrl+c", "q":
			return m, tea.Quit

		// Reload the configuration file
		case "r":
			m.message = "Reloading config file... (not)."

		// Help
		case "h":
			m.showingHelp = !m.showingHelp

		// all others
		default:
			showingOutputRow := -1
			for index := range m.programs {
				programNum := index + 1
				if m.programs[index].StartStopChar == ch {
					var err error
					debug.DumpStringToDebugListener(fmt.Sprintf("Sending start/stop to program %d\n", programNum))
					m.message, err = m.programs[index].StartStopProgram(p)
					if err == nil {
						m.programs[index].ProgramFinalMessage = fmt.Sprintf("Program %d running...\n", programNum)
						debug.DumpStringToDebugListener(fmt.Sprintf("Finished sending start/stop to program %d\n", programNum))
						return m, nil
					} else {
						m.programs[index].ProgramRunning = false
						m.programs[index].ProgramSuccess = false
						m.message = fmt.Sprintf("Starting program %d got error: %v\nOutput: %s\n", programNum, err, m.message)
					}
				} else if m.programs[index].ViewOutputChar == ch {
					showingOutputRow = index
					m.message = m.programs[index].ProgramFinalMessage + "\n"
					//m.message += "Stdout:\n"
					//for s := range m.programs[index].ProgramStdOut.Iter() {
					//	m.message += s + "\n"
					//}
					//m.message += "\nStderr:\n"
					//for s := range m.programs[index].ProgramStdErr.Iter() {
					//	m.message += s + "\n"
					//}
					stdOut := ""
					for s := range m.programs[index].ProgramStdOut.Iter() {
						//stdOut += s + "\n"
						stdOut += wrap.String(s+"\n", m.outViewport.Width)
					}
					//stdErr := ""
					//for s := range m.programs[index].ProgramStdErr.Iter() {
					//	stdErr += s + "\n"
					//}
					m.outViewport.SetContent(stdOut)
				}
			}

			if showingOutputRow != -1 {
				for index := range m.programs {
					m.programs[index].ShowingOutputNow = index == showingOutputRow
				}
			}
		}
	}

	// Handle keyboard and mouse events in the viewport
	m.outViewport, cmd = m.outViewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}

	if m.showingHelp {
		return m.helpView()
	}

	// Send the UI for rendering
	return fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n",
		m.titleView(), m.headerView(), m.outputTitleView(), m.outViewport.View(), m.footerView())
}

var p *tea.Program

func main() {
	model := initialModel()
	p = tea.NewProgram(model, tea.WithAltScreen())
	if err := p.Start(); err != nil {
		debug.DumpStringToDebugListener(fmt.Sprintf("Alas, there's been an error: %v", err))
		fmt.Printf("Alas, there's been an error: %v\n", err)
		os.Exit(1)
	}
	if globalError != nil {
		debug.DumpStringToDebugListener(fmt.Sprintf("Alas, there's been an error: %v", globalError))
		fmt.Printf("Alas, there's been an error: %v\n", globalError)
		os.Exit(1)
	}
	fmt.Println("Done!")
}
