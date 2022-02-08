package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Application constants, defining host, port, and protocol.
const (
	debugListenerHost = "localhost"
	debugListenerPort = "21212"
	debugListenerType = "tcp"
)

func dumpStringToDebugListener(output string) {
	conn, err := net.Dial(debugListenerType, debugListenerHost+":"+debugListenerPort)
	if err != nil {
		return
	}
	defer conn.Close()

	// Send to socket connection.
	_, _ = conn.Write([]byte(output + "\n"))
}

type configuration struct {
	Name     string
	Commands []programState
}

type programState struct {
	ProgramCommand string
	ProgramRunning bool
	ProgramRan     bool
	ProgramSuccess bool
	ProgramOutput  string
	StartStopChar  string
	ViewOutputChar string
}

type model struct {
	err             error
	waitingOnConfig bool
	showingHelp     bool
	message         string
	programs        []programState
}

type programFinishedMessage struct {
	programSuccess bool
	programOutput  string
}

var globalError error

func loadConfigFile() tea.Msg {
	file, err := ioutil.ReadFile("config.json")

	if err != nil {
		dumpStringToDebugListener(fmt.Sprintln("... err on reading config.json is", err))
		return errMsg{err}
	}

	var data configuration
	err = json.Unmarshal([]byte(file), &data)

	if err != nil {
		dumpStringToDebugListener(fmt.Sprintln("... err on unmarshalling json is", err))
		return errMsg{err}
	}

	return data
}

type errMsg struct{ err error }

// For messages that contain errors it's often handy to also implement the
// error interface on the message.
func (e errMsg) Error() string { return e.err.Error() }

func initialModel() model {
	return model{
		waitingOnConfig: true,
		programs:        make([]programState, 0),
	}
}

func startProgram(m *programState) {
	go func() {
		commandAndArgs := strings.Split(m.ProgramCommand, " ")

		var stdOut bytes.Buffer
		var stdErr bytes.Buffer
		runCommand := &exec.Cmd{
			Path:   commandAndArgs[0],
			Args:   commandAndArgs,
			Stdout: &stdOut,
			Stderr: &stdErr,
		}

		err := runCommand.Run()
		message := programFinishedMessage{}
		if err != nil {
			message.programOutput = strings.TrimSpace(string(stdErr.Bytes()))
			message.programSuccess = false
		} else {
			message.programOutput = strings.TrimSpace(string(stdOut.Bytes()))
			message.programSuccess = true
		}

		p.Send(message)
	}()
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
	s += "-----------+-------------+----------+--------\n"

	for index := range m.programs {
		runningState := " "
		if m.programs[index].ProgramRunning {
			runningState = "Y"
		} else if m.programs[index].ProgramRan && !m.programs[index].ProgramSuccess {
			runningState = "Error!"
		}

		command := m.programs[index].ProgramCommand
		if strings.Index(command, "/") == 0 {
			args := strings.Split(command, " ")
			programParts := strings.Split(args[0], "/")
			command = programParts[len(programParts)-1] + " " + strings.Join(args[1:], " ")
		}

		s += fmt.Sprintf(" %-10s|  %-11s|  %-8s| %s\n",
			m.programs[index].StartStopChar, m.programs[index].ViewOutputChar,
			runningState, command)
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

	case configuration:
		m.waitingOnConfig = false
		m.programs = msg.Commands
		startStop := '1'
		view := 'a'
		for index := range m.programs {
			m.programs[index].StartStopChar = string(startStop)
			m.programs[index].ViewOutputChar = string(view)
			startStop += 1
			view += 1
			if view == 'q' {
				view = 's'
			}
		}
		return m, nil

	case errMsg:
		dumpStringToDebugListener(fmt.Sprintln("... update got an errMsg with error", msg))
		globalError = msg
		return m, tea.Quit

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
			m.message = "Reloading config file..."

		// Help
		case "h":
			m.showingHelp = !m.showingHelp

		// all others
		default:
			for index := range m.programs {
				if m.programs[index].StartStopChar == ch {
					m.message = fmt.Sprintf("Starting/stopping program %d\n", index)
					m.programs[index].ProgramRunning = !m.programs[index].ProgramRunning
				} else if m.programs[index].ViewOutputChar == ch {
					m.message = fmt.Sprintf("Viewing output of program %d\n", index)
				}
			}
		}

		// Notification that the program finished
		//case programFinishedMessage:
		//	m.programRunning = false
		//	m.programSuccess = msg.programSuccess
		//	m.programOutput = msg.programOutput
		//	m.programRan = true
		//	return m, nil
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
		dumpStringToDebugListener(fmt.Sprintf("Alas, there's been an error: %v", err))
		fmt.Printf("Alas, there's been an error: %v\n", err)
		os.Exit(1)
	}
	if globalError != nil {
		dumpStringToDebugListener(fmt.Sprintf("Alas, there's been an error: %v", globalError))
		fmt.Printf("Alas, there's been an error: %v\n", globalError)
		os.Exit(1)
	}
	fmt.Println("Done!")
}
