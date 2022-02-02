package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type model struct {
	programCommand string
	programRunning bool
	programRan     bool
	programSuccess bool
	programOutput  string
}

type programFinishedMessage struct {
	programSuccess bool
	programOutput  string
}

func initialModel() model {
	return model{
		programCommand: "/usr/local/bin/go build",
		programRunning: false,
		programRan:     false,
		programSuccess: false,
		programOutput:  "",
	}
}

func startProgram(m *model) {
	go func() {
		commandAndArgs := strings.Split(m.programCommand, " ")

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

func (m model) Init() tea.Cmd {
	// Just return `nil`, which means "no I/O right now, please."
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// Is it a key press?
	case tea.KeyMsg:
		// Cool, what was the actual key pressed?
		switch msg.String() {

		// Run the program
		case "r":
			m.programRan = false
			if !m.programRunning {
				m.programRunning = true
				startProgram(&m)
			}
			return m, nil

		// These keys should exit the program.
		case "ctrl+c", "q":
			return m, tea.Quit
		}

	// Notification that the program finished
	case programFinishedMessage:
		m.programRunning = false
		m.programSuccess = msg.programSuccess
		m.programOutput = msg.programOutput
		m.programRan = true
		return m, nil
	}

	// Return the updated model to the Bubble Tea runtime for processing.
	// Note that we're not returning a command.
	return m, nil
}

func (m model) View() string {
	s := "Program runner.\n\n"
	s += "Press r to run the program.\n\n"
	s += "Current command: " + m.programCommand + "\n\n"

	if m.programRunning {
		s += "...program is running...\n"
	}

	if m.programRan {
		if m.programSuccess {
			s += "Success!\n"
		} else {
			s += "Failure!\n"
		}

		if len(m.programOutput) > 0 {
			s += "\n" + m.programOutput + "\n"
		}
	}

	s += "\nPress q to quit.\n"

	// Send the UI for rendering
	return s
}

var p *tea.Program

func main() {
	p = tea.NewProgram(initialModel(), tea.WithAltScreen())
	if err := p.Start(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
