package process

import (
	"bufio"
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"charm_runner/circular_buffer"
	"charm_runner/debug"

	tea "github.com/charmbracelet/bubbletea"
)

type ProgramState struct {
	ProgramIndex        int
	ProgramCommand      string
	ProgramRunning      bool
	ProgramRan          bool
	ProgramSuccess      bool
	ProgramFinalMessage string
	ProgramStdOut       circular_buffer.CircularBuffer
	ProgramStdErr       circular_buffer.CircularBuffer
	StartStopChar       string
	ViewOutputChar      string
	Process             exec.Cmd
}

type ProgramFinishedMessage struct {
	ProgramIndex   int
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
			msg := "Can't create StdoutPipe: " + err.Error()
			m.ProgramStdErr.AddString(msg)
			debug.DumpStringToDebugListener(msg)
			return
		}

		var wg sync.WaitGroup

		stdOutChan := make(chan string, 1)
		stdOutDone := make(chan bool, 1)
		go func(wg *sync.WaitGroup) {
			defer wg.Done()

			scanner := bufio.NewScanner(stdOut)
			for scanner.Scan() {
				stdOutChan <- string(scanner.Bytes())
			}

			debug.DumpStringToDebugListener("Ran out of stdout input, read thread bailing.")
			stdOutDone <- true
		}(&wg)

		wg.Add(1)

		stdErr, err := runCommand.StderrPipe()
		if err != nil {
			msg := "Can't create StderrPipe: " + err.Error()
			m.ProgramStdErr.AddString(msg)
			debug.DumpStringToDebugListener(msg)
		}

		stdErrChan := make(chan string, 1)
		stdErrDone := make(chan bool, 1)
		go func(wg *sync.WaitGroup) {
			defer wg.Done()

			scanner := bufio.NewScanner(stdErr)
			for scanner.Scan() {
				stdErrChan <- string(scanner.Bytes())
			}

			debug.DumpStringToDebugListener("Ran out of stderr input, read thread bailing.")
			stdErrDone <- true
		}(&wg)

		wg.Add(1)

		err = runCommand.Start()
		message := ProgramFinishedMessage{
			ProgramIndex: m.ProgramIndex,
		}
		if err != nil {
			message.ProgramOutput = fmt.Sprintf("Program %d failed with error:\n  %v\n", m.ProgramIndex+1, err.Error())
			message.ProgramSuccess = false
		} else {
			message.ProgramOutput = fmt.Sprintf("Program %d finished successfully.", m.ProgramIndex+1)
			message.ProgramSuccess = true
		}

		keepGoingOut := true
		keepGoingErr := true
		for keepGoingOut || keepGoingErr {
			select {
			case res, isOpen := <-stdOutChan:
				if !isOpen {
					if keepGoingOut {
						debug.DumpStringToDebugListener("stdOutChan is no longer open.")
					}
				} else {
					m.ProgramStdOut.AddString(res)
				}

			case res, isOpen := <-stdErrChan:
				if !isOpen {
					if keepGoingErr {
						debug.DumpStringToDebugListener("stdErrChan is no longer open.")
					}
				} else {
					m.ProgramStdErr.AddString(res)
				}

			case <-stdOutDone:
				keepGoingOut = false

			case <-stdErrDone:
				keepGoingErr = false
			}
		}

		wg.Wait()

		if err := runCommand.Wait(); err != nil {
			debug.DumpStringToDebugListener(fmt.Sprintf("Error waiting for command execution: %s\n", err.Error()))
		}

		debug.DumpStringToDebugListener("Program.Run finished, final message is: " + message.ProgramOutput)
		p.Send(message)
	}()
}

func (prog *ProgramState) StartStopProgram(p *tea.Program) (string, error) {
	debug.DumpStringToDebugListener("Entering StartStopProgram")
	var msg string
	if prog.ProgramRunning {
		msg = fmt.Sprintf("Stopping program %s\n", prog.ProgramCommand)
	} else {
		msg = fmt.Sprintf("Starting program %s\n", prog.ProgramCommand)
		startProgram(prog, p)
	}

	prog.ProgramRunning = !prog.ProgramRunning
	debug.DumpStringToDebugListener(msg)
	return msg, nil
}
