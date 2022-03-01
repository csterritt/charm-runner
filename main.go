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
	"charm_runner/types"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type configuration struct {
	Name     string
	Commands []process.ProgramState
}

type model struct {
	err             error
	waitingOnConfig bool
	showingHelp     bool
	message         string
	programs        []process.ProgramState
}

type errMsg struct{ err error }

// For messages that contain errors it's often handy to also implement the
// error interface on the message.
func (e errMsg) Error() string { return e.err.Error() }

var (
	globalError error

	errorStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("9"))
)

func loadConfigFile() tea.Msg {
	file, err := ioutil.ReadFile("config.json")

	if err != nil {
		debug.DumpStringToDebugListener(fmt.Sprintln("... err on reading config.json is", err))
		return errMsg{err}
	}

	var data configuration
	err = json.Unmarshal([]byte(file), &data)

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

func helpView(m model) string {
	s := "Program runner help:\n\nPress h again to exit help.\n\n"

	return s
}

func mainView(m model) string {
	s := "Program runner.\n\nPress:\n  h for help\n  q to quit.\n  r to reload the configuration.\n\n"

	if m.err != nil {
		s += "Error found: " + m.err.Error() + "\n\n"
	}

	s += "Start/Stop | View output | Running? | Program\n"
	s += "-----------+-------------+----------+---------\n"

	for index := range m.programs {
		runningState := " "
		showError := false
		if m.programs[index].ProgramRunning {
			runningState = "Y"
		} else if m.programs[index].ProgramRan && !m.programs[index].ProgramSuccess {
			runningState = "Error!"
			showError = true
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
		s += fmt.Sprintf(" %-10s|  %-11s|%s| %s\n",
			m.programs[index].StartStopChar, m.programs[index].ViewOutputChar,
			runningStateOut, command)
	}

	s += "\n" + m.message + "\n"

	// Send the UI for rendering
	return s
}

func (m model) Init() tea.Cmd {
	return loadConfigFile
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		return m, nil

	case types.InfoMessage:
		m.message = msg.Message
		debug.DumpStringToDebugListener("Got message " + msg.Message + " in Update.")
		return m, nil

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
					m.message = m.programs[index].ProgramFinalMessage + "\n"
					m.message += "Stdout:\n"
					for s := range m.programs[index].ProgramStdOut.Iter() {
						m.message += s + "\n"
					}
					m.message += "\nStderr:\n"
					for s := range m.programs[index].ProgramStdErr.Iter() {
						m.message += s + "\n"
					}
				}
			}
		}
	}

	// Return the updated model to the Bubble Tea runtime for processing.
	// Note that we're not returning a command.
	return m, nil
}

func (m model) View() string {
	var s string
	if m.showingHelp {
		s = helpView(m)
	} else {
		s = mainView(m)
	}

	// Send the UI for rendering
	return s
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
