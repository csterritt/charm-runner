package process

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"charm_runner/debug"

	tea "github.com/charmbracelet/bubbletea"
)

type ProgramState struct {
	ProgramCommand string
	ProgramRunning bool
	ProgramRan     bool
	ProgramSuccess bool
	ProgramStdOut  io.Reader
	ProgramStdErr  io.Reader
	StartStopChar  string
	ViewOutputChar string
	Process        exec.Cmd
}

type ProgramFinishedMessage struct {
	ProgramSuccess bool
	ProgramOutput  string
}

func startProgram(m *ProgramState, p *tea.Program) {
	go func() {
		commandAndArgs := strings.Split(m.ProgramCommand, " ")

		runCommand := &exec.Cmd{
			Path: commandAndArgs[0],
			Args: commandAndArgs,
		}
		stdOut, err := runCommand.StdoutPipe()
		if err != nil {
			fmt.Println("Can't create StdoutPipe:", err)
			os.Exit(1)
		}

		stdOutChan := make(chan string, 1)
		go func() {
			scanner := bufio.NewScanner(stdOut)
			for scanner.Scan() {
				stdOutChan <- string(scanner.Bytes())
			}
			fmt.Println("Ran out of stdout input, read thread bailing.")
			close(stdOutChan)
		}()

		err = runCommand.Run()
		message := ProgramFinishedMessage{}
		if err != nil {
			message.ProgramOutput = "bogus"
			message.ProgramSuccess = false
		} else {
			message.ProgramOutput = "bogus"
			message.ProgramSuccess = true
		}

		p.Send(message)
	}()
}

func (prog *ProgramState) StartStopProgram() string {
	debug.DumpStringToDebugListener("Entering StartStopProgram")
	var msg string
	if prog.ProgramRunning {
		msg = fmt.Sprintf("Stopping program %s\n", prog.ProgramCommand)
	} else {
		msg = fmt.Sprintf("Starting program %s\n", prog.ProgramCommand)
	}

	prog.ProgramRunning = !prog.ProgramRunning
	debug.DumpStringToDebugListener(msg)
	return msg
}
