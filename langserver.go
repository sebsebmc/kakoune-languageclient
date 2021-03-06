package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/sourcegraph/jsonrpc2"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type Position struct {
	Line      int
	Character int
}

type textRange struct {
	Start Position
	End   Position
}

type ioReadWriteCloser struct {
	io.ReadCloser
	io.WriteCloser
}

type MarkedString markedString

type markedString struct {
	Language string `json:"language,omitempty"`
	Value    string `json:"value,omitempty"`
	Simple   string `json:"simple,omitempty"`
}

func (m *MarkedString) UnmarshalJSON(data []byte) error {
	if d := strings.TrimSpace(string(data)); len(d) > 0 && d[0] == '"' {
		// Raw string
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		m.Value = s
		return nil
	}
	// Language string
	ms := (*markedString)(m)
	return json.Unmarshal(data, ms)
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
	return rw.WriteCloser.Write(buf)
}

//Proxies reading in, so it may have to remove the header, which may or may not be present...
func (rw ioReadWriteCloser) Read(p []byte) (int, error) {
	n, err := rw.ReadCloser.Read(p)
	fmt.Printf("<-- %s\n", string(p))
	return n, err
}

type LangSrvr struct {
	conn     *jsonrpc2.Conn
	handlers map[string]func(*kakBuffer, []string) string
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
	lspRPC.handlers = make(map[string](func(*kakBuffer, []string) string))
	lspRPC.conn = jsonrpc2.NewConn(context.Background(), jsonrpc2.NewBufferedStream(serverConn, jsonrpc2.VSCodeObjectCodec{}),
		lspRPC)
	return &lspRPC
}

func (ls *LangSrvr) Initialize() {
	wd, _ := os.Getwd()
	fileUri := "file://" + wd
	params := map[string]interface{}{"processId": os.Getpid(), "rootUri": fileUri, "rootPath": wd,
		"capabilities": make(map[string]interface{})}
	//TODO: use capabilities to assign handlers
	ls.handlers["textDocument/sync"] = ls.tdSync
	ls.handlers["textDocument/hover"] = ls.tdHover
	ls.handlers["textDocument/signatureHelp"] = ls.tdSigHelp
	ls.execCommandSync("initialize", params, nil)
	ls.notify("initialized", nil)
}

func (ls LangSrvr) HandleKak(buf *kakBuffer, cmd *lspCommand) (string, error) {
	handler, ok := ls.handlers[cmd.command]
	if ok == false {
		return "", errors.New("Command does not exist")
	}
	return handler(buf, cmd.args), nil
}

// This function receives messages from the LS
func (ls LangSrvr) Handle(c context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {

}

func (ls LangSrvr) Shutdown() {
	ls.execCommandSync("shutdown", map[string]interface{}{}, nil)
}

func (ls LangSrvr) execCommandSync(command string, params map[string]interface{}, reply interface{}) error {
	err := ls.conn.Call(context.Background(), command, params, reply)
	if err != nil {
		fmt.Printf("Err1(): %q\n", err)
		return errors.New("RPC Error")
	}
	return nil
}

func (ls LangSrvr) notify(method string, args interface{}) {
	ls.conn.Notify(context.Background(), method, args)
}

//=============== Handlers =================================

// Provides syncing interface, however we can only sync the whole file because
// Kakoune doesn't provide the changes since last save. The current method
// involves telling Kakoune to write the buffer to a temp file and copying
// the contents of the temp file. Eww...
func (ls *LangSrvr) tdSync(buf *kakBuffer, params []string) string {
	contents, err := ioutil.ReadAll(buf.tmpFile)
	if err != nil {
    	fmt.Println("Failed to read temp file for syncing")
	}
	if buf.lastSync == 0 {
		ls.notify("textDocument/didOpen", map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri":      buf.file,
				"version":  buf.lastEdit,
				"language": buf.language,
				"text":     contents,
			},
		})
	} else {
		ls.notify("textDocument/didChange", map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri":     buf.file,
				"version": buf.lastEdit,
			},
			"contentChanges": []map[string]interface{}{
				0: {"text": contents},
			},
		})
	}
	buf.lastSync = buf.lastEdit
	return "nop" //Oh look, a use for nop
}

// textDocument/hover requires syncing
// buffile, line, charachter
// textDocument/hover
// params:{textDocument:URI, position:{line:,character:}}
func (ls *LangSrvr) tdHover(buf *kakBuffer, params []string) string {
	fmt.Printf("hover: %s\n", params)
	uri := "file://" + buf.file
	line, _ := strconv.Atoi(params[0])
	character, _ := strconv.Atoi(params[1])
	paramMap := map[string]interface{}{
		"textDocument": map[string]string{"uri": uri},
		"position": map[string]interface{}{
			"line":      line - 1,
			"character": character - 1,
		},
	}
	reply := struct {
		Docs   []MarkedString `json:"contents"`
		Ranges textRange      `json:"range,omitempty"`
	}{}
	err := ls.execCommandSync("textDocument/hover", paramMap, &reply)
	if err != nil {
		return "echo 'Command Failed'"
	}
	fmt.Println(reply)
	return fmt.Sprintf("info -placement below -anchor %s.%s '%s'", params[0], params[1], reply.Docs[0].Value)
}

// textDocument/signatureHelp requires syncing
// params:{textDocument:URI, position:{line:#,character:#}}
func (ls LangSrvr) tdSigHelp(buf *kakBuffer, params []string) string {
	fmt.Printf("sigHelp: %s\n", params)

	uri := "file://" + buf.file 
	line, _ := strconv.Atoi(params[0])
	character, _ := strconv.Atoi(params[1])
	paramMap := map[string]interface{}{
		"textDocument": map[string]string{"uri": uri},
		"position": map[string]interface{}{
			"line":      line - 1,
			"character": character - 1,
		},
	}
	type sigInfo struct {
		Label  string                   `json:"label"`
		Docs   string                   `json:"documentation,omitempty"`
		Params []map[string]interface{} `json:"parameters,omitempty"`
	}
	reply := struct {
		Signatures []sigInfo `json:"signatures"`
		AParam     int       `json:"activeParameter,omitempty"`
		ASig       int       `json:"activeSignature,omitempty"`
	}{}
	err := ls.execCommandSync("textDocument/signatureHelp", paramMap, &reply)
	if err != nil {
		return "echo 'Command failed'"
	}
	if len(reply.Signatures) == 0 {
		return "echo 'No signatures found'"
	}
	for i := 0; i < len(reply.Signatures); i++ {
		s := reply.Signatures[i]
		s.Docs = strings.Replace(s.Docs, "\\'", "\\\\'", -1)
		s.Docs = strings.Replace(s.Docs, "'", "\\'", -1)
		s.Docs = strings.Replace(s.Docs, "\"", "\\\"", -1)
		reply.Signatures[i] = s
	}
	fmt.Println(reply)
	return fmt.Sprintf("info -placement below -anchor %s.%s '%s\n%s'", params[0], params[1], reply.Signatures[reply.ASig].Label, reply.Signatures[reply.ASig].Docs)
}
