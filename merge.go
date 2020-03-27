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
	content     string
	sender      int
	destination *os.File
}

func (output message) String() string {
	if isTerminal(output.destination) {
		return fmt.Sprintf(COLOR_FORMAT, (output.sender%NUM_COLORS)+1, output.content)
	}
	return output.content
}

var isTerminalCache = make(map[*os.File]bool) // Avoid making a Stat syscall for every message

func isTerminal(file *os.File) bool {
	if fileIsTerminal, ok := isTerminalCache[file]; ok {
		return fileIsTerminal
	}
	info, err := file.Stat()
	isTerminalCache[file] = (err == nil) && (info.Mode()&os.ModeCharDevice != 0)
	return isTerminalCache[file]
}

func readPipe(pipe io.ReadCloser, destination *os.File, id int, mergedOutput chan message, wait *sync.WaitGroup) {
	defer wait.Done()
	output := bufio.NewScanner(pipe)
	for output.Scan() {
		mergedOutput <- message{TRIM_COLORS_PATTERN.ReplaceAllString(output.Text(), ""), id, destination}
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
func mergeOutErr(command *exec.Cmd, id int, mergedOutput chan message) {
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
		mergedOutput <- message{fmt.Sprintf("Started '%s' (%d)", identifier(command), command.Process.Pid), id, os.Stdout}
	}
	var both sync.WaitGroup
	both.Add(2)
	go readPipe(stdout, os.Stdout, id, mergedOutput, &both)
	go readPipe(stderror, os.Stderr, id, mergedOutput, &both)
	both.Wait()
	close(mergedOutput)
}

func listenTo(command *exec.Cmd, id int) {
	defer processWait.Done()
	mergedOutput := make(chan message)
	go mergeOutErr(command, id, mergedOutput)
	for message := range mergedOutput {
		messageBuffer <- message
	}
	if err, ok := (command.Wait()).(*exec.ExitError); err != nil {
		if ok {
			messageBuffer <- message{fmt.Sprintf("Process '%s' (%d) exited with status %d", identifier(command), command.Process.Pid, err.ExitCode()), id, os.Stdout}
		} else {
			messageBuffer <- message{fmt.Sprintf("Process '%s' (%d) exited abnormally. Details: %s", identifier(command), command.Process.Pid, err), id, os.Stdout}
		}
	} else {
		messageBuffer <- message{fmt.Sprintf("Process '%s' (%d) exited successfully", identifier(command), command.Process.Pid), id, os.Stdout}
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
		fmt.Fprintln(output.destination, output)
	}
}
