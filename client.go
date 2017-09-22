package main

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/mafredri/cdp/rpcc"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

type kakInstance struct {
	session, client, pipe string
}

type languageServer struct {
	server *rpcc.Conn
}

func main() {
	session := os.Args[1]
	client := os.Args[2]
	tmpDir, _ := ioutil.TempDir("", "kakoune-languageclient")

	namedPipe := filepath.Join(tmpDir, session)
	syscall.Mkfifo(namedPipe, 0600)

	instance := kakInstance{session, client, namedPipe}

	//hold the write end of the pipe so we dont get EOF
	fifo, _ := os.OpenFile(namedPipe, os.O_RDWR, os.ModeNamedPipe)

	//Create pipe first, then let Kakoune know about it
	instance.execCommand(fmt.Sprintf("decl str lsc_pipe %s", namedPipe))
	//pr, _ := io.Pipe()
	reader := bufio.NewReader(fifo)
	for {
		line, _, err := reader.ReadLine()
		if err == io.EOF {
			break
		}
		if bytes.Compare(line, []byte("Ping")) == 0 {
			instance.execCommand("echo -debug Pong\n")
		}
	}
}

func (inst *kakInstance) execCommand(command string) {
	cmd := exec.Command("kak", "-p", inst.session)
	in, err := cmd.StdinPipe()
	if err != nil {
		fmt.Println("Failed to get the stdin pipe!")
	}
	cmd.Stdout = os.Stdout
	//There is no Stdout for a -p
	cmd.Start()
	in.Write([]byte(fmt.Sprintf("eval -client %s %s", inst.client, command)))
	in.Close()
	cmd.Wait()
}
