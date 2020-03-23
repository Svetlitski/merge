package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
)

const COLOR_FORMAT string = "\033[3%dm%s\033[0m"
const NUM_COLORS = 6

var TRIM_COLORS_PATTERN *regexp.Regexp

var messageBuffer = make(chan message)
var processWait sync.WaitGroup

type message struct {
	content string
	sender  int
}

func (output message) String() string {
	return fmt.Sprintf(COLOR_FORMAT, (output.sender%NUM_COLORS)+1, output.content)
}

func readPipe(pipe io.ReadCloser, mergedOutput chan string, wait *sync.WaitGroup) {
	defer wait.Done()
	output := bufio.NewScanner(pipe)
	for output.Scan() {
		mergedOutput <- TRIM_COLORS_PATTERN.ReplaceAllString(output.Text(), "")
	}
}

func identifier(command *exec.Cmd) string {
	fullId := strings.Join(command.Args, " ")
	if len(fullId) <= 60 {
		return fullId
	}
	return fullId[:60] + "..."
}

// Sends both stdout and stderror of command to mergedOutput
func mergeOutErr(command *exec.Cmd, mergedOutput chan string) {
	stdout, err := command.StdoutPipe()
	if err != nil {
		fatal(fmt.Sprintf("Could not connect to stdout of process '%s'. Details: %s", identifier(command), err))
	}
	stderror, err := command.StderrPipe()
	if err != nil {
		fatal(fmt.Sprintf("Could not connect to stderr of process '%s'. Details: %s", identifier(command), err))
	}
	if err := command.Start(); err != nil {
		fatal(fmt.Sprintf("Could not start process '%s'. Details: %s", identifier(command), err))
	} else {
		mergedOutput <- fmt.Sprintf("Started '%s' (%d)", identifier(command), command.Process.Pid)
	}
	var both sync.WaitGroup
	both.Add(2)
	go readPipe(stdout, mergedOutput, &both)
	go readPipe(stderror, mergedOutput, &both)
	both.Wait()
	close(mergedOutput)
}

func listenTo(command *exec.Cmd, id int) {
	defer processWait.Done()
	mergedOutput := make(chan string)
	go mergeOutErr(command, mergedOutput)
	for line := range mergedOutput {
		messageBuffer <- message{line, id}
	}
	if err, ok := (command.Wait()).(*exec.ExitError); err != nil {
		if ok {
			messageBuffer <- message{fmt.Sprintf("Process '%s' (%d) exited with status %d", identifier(command), command.Process.Pid, err.ExitCode()), id}
		} else {
			messageBuffer <- message{fmt.Sprintf("Process '%s' (%d) exited abnormally. Details: %s", identifier(command), command.Process.Pid, err), id}
		}
	} else {
		messageBuffer <- message{fmt.Sprintf("Process '%s' (%d) exited successfully", identifier(command), command.Process.Pid), id}
	}
}

func fatal(errorMessage string) {
	fmt.Fprintln(os.Stderr, fmt.Sprintf("Error: %s", errorMessage))
	os.Exit(1)
}

func main() {
	if len(os.Args) < 3 {
		fatal("Must supply at least two processes to run")
	}
	TRIM_COLORS_PATTERN = regexp.MustCompile("\033\\[[^m]*m")
	processWait.Add(len(os.Args) - 1)
	for i, cmd := range os.Args[1:] {
		fields := strings.Fields(cmd)
		go listenTo(exec.Command(fields[0], fields[1:]...), i)
	}
	go (func() {
		processWait.Wait()
		close(messageBuffer)
	})()
	for output := range messageBuffer {
		fmt.Println(output)
	}
}
