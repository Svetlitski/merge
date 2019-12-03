package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
)

const COLOR_FORMAT string = "\033[3%dm%s\033[0m"
const NUM_COLORS int = 6

type message struct {
	content string
	sender  int
}

func (output message) String() string {
	return fmt.Sprintf(COLOR_FORMAT, (output.sender%NUM_COLORS)+1, output.content)
}

var messageBuffer = make(chan message)
var exitCount int = 0
var numProcs int
var exitLock sync.Mutex

func readPipe(pipe io.ReadCloser, commandBuffer chan string, wait *sync.WaitGroup) {
	output := bufio.NewScanner(pipe)
	for output.Scan() {
		commandBuffer <- output.Text()
	}
	wait.Done()
}

// Sends both stdout and stderror of command to commandBuffer
func bufferOutput(command *exec.Cmd, commandBuffer chan string, start chan bool) {
	stdoutPipe, err := command.StdoutPipe()
	if err != nil {
		panic("Could not connect to stdout of process " + command.Path)
	}
	stderrorPipe, err := command.StderrPipe()
	if err != nil {
		panic("Could not connect to stderror of process " + command.Path)
	}
	start <- true
	var both sync.WaitGroup
	both.Add(2)
	go readPipe(stdoutPipe, commandBuffer, &both)
	go readPipe(stderrorPipe, commandBuffer, &both)
	both.Wait()
	close(commandBuffer)
}

func listenTo(command *exec.Cmd, id int) {
	commandBuffer := make(chan string)
	start := make(chan bool)
	go bufferOutput(command, commandBuffer, start)
	<-start
	if err := command.Start(); err != nil {
		panic(fmt.Sprintf("Could not start process '%s'. Error: %s", command.Path, err))
	}
	for output := range commandBuffer {
		messageBuffer <- message{output, id}
	}
	//command.Wait()
	exitLock.Lock()
	exitCount++
	if exitCount == numProcs {
		close(messageBuffer)
	}
	exitLock.Unlock()
}

func main() {
	numProcs = len(os.Args) - 1
	if numProcs < 2 {
		panic("Must supply at least two processes to run")
	}
	for i, cmd := range os.Args[1:] {
		fields := strings.Fields(cmd)
		go listenTo(exec.Command(fields[0], fields[1:]...), i)
	}
	for output := range messageBuffer {
		fmt.Println(output)
	}
}
