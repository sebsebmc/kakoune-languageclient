# kakoune-languageclient

## About

Small implementation of a [language client](https://github.com/microsoft/language-server-protocol) for [Kakoune](https://github.com/mawww/kakoune).

Uses a small Go binary to handle LSP JSONRPC translation to Kakoune commands.

## Design

There are 3 parts to using LSP for Kakoune: Kakoune itself, a helper binary, and the language servers. Since the protocol communicates using JSON RPC 2.0 a helper binary is used manage the language servers and parsing the JSON.

The helper binary interfaces with Kakoune by writing commands into a fifo file that is being read by the binary, the binary then translates the commands into LSP commands and talks to the language servers. Finally, the binary connects to the running Kakoune instance and executes Kakoue commands to render results/perform the requested actions.

The setup process can be automated using a kak config file, the current setup is in lsc.kak. The language-client is currently not being started automatically due to stability issues, and requires pressing 0  to start.

## Development

Currently the server is hardcoded to run only the [go language-server](https://github.com/sourcegraph/go-langserver) for testing. One of the next tasks will be to add configurations into the config file to handle language servers for more languages.

To aid in debugging the binary there is a manual launch method that can be used to display the output from the binary. The recommended method is to launch a Kakoune instance in one terminal, and then in another terminal launch the go binary, passing the session and client name (These values are shown in the bottom right of Kakoune by default). This way you can use print statements, etc. from the binary to help in debugging.