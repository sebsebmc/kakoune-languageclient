package langsrvr

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/powerman/rpc-codec/jsonrpc2"
	"io"
	"net/rpc"
	"os"
	"os/exec"
)

type ioReadWriteCloser struct {
	io.ReadCloser
	io.WriteCloser
}

//From https://github.com/natefinch/pie
func (rw ioReadWriteCloser) Close() error {
	err := rw.ReadCloser.Close()
	if err := rw.WriteCloser.Close(); err != nil {
		return err
	}
	return err
}
func (rw ioReadWriteCloser) Write(buf []byte) (int, error) {
	fmt.Printf("--> %s\n", string(buf))
	contentLength := len(buf)
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", contentLength)
	rw.WriteCloser.Write([]byte(header))
	n, err := rw.WriteCloser.Write(buf)
	return n, err
}

func (rw ioReadWriteCloser) Read(p []byte) (int, error) {
	headerReader := bufio.NewReader(rw.ReadCloser)
	header, _, _ := headerReader.ReadLine()
	fmt.Println(string(header))
	next, _ := headerReader.Peek(1)
	fmt.Printf("Next 2: %v", next)
	for bytes.Compare(next, []byte("{")) == 0 {
		header, _, _ =headerReader.ReadLine()
		fmt.Println(header)
		next, _ = headerReader.Peek(1)
	}
	n, err := rw.ReadCloser.Read(p)
	fmt.Printf("<-- %s\n", string(p))
	return n, err
}

type LangSrvr struct {
	conn *jsonrpc2.Client
}

func NewLangSrvr(command string) *LangSrvr {
	//Make a go-langserver to test
	server := exec.Command(command)
	fmt.Println("Starting langserver!")
	srvIn, err := server.StdinPipe()
	if err != nil {
		fmt.Printf("%s\n", err)
	}
	srvOut, err := server.StdoutPipe()
	if err != nil {
		fmt.Printf("%s\n", err)
	}
	serverConn := ioReadWriteCloser{srvOut, srvIn}
	err = server.Start()
	if err != nil {
		fmt.Printf("%s\n", err)
	}
	var lspRPC LangSrvr
	lspRPC.conn = jsonrpc2.NewClient(serverConn)
	return &lspRPC
}

func (ls *LangSrvr) Initialize() {
	var reply map[string]interface{}
	/*initCall := lspRPC.conn.Go("initialize",
	map[string]interface{}{"processId": os.Getpid(), "rootUri": "file:///home/seb/",
		"capabilities": make(map[string]interface{})},
	&reply, nil)*/

	err := ls.conn.Call("initialize",
		map[string]interface{}{"processId": os.Getpid(), "rootUri": "file:///home/seb/",
			"capabilities": make(map[string]interface{})},
		&reply)
	//doneCall := <-initCall.Done
	if err == rpc.ErrShutdown || err == io.ErrUnexpectedEOF {
		fmt.Printf("Err1(): %q\n", err)
	} else if err != nil {
		rpcerr := jsonrpc2.ServerError(err)
		fmt.Printf("Err1(): code=%d msg=%q data=%v\n", rpcerr.Code, rpcerr.Message, rpcerr.Data)
	}
}
