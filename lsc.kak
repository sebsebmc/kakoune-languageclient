echo -debug "Loading lsc.kak"

#Welp, this is actually becoming a problem... Move configs into an actual config file?
decl -docstring "Max filesize to auto-sync in KiB" int lsc_max_sync_size 1024

#Mapping filetype to command to launch a language server
decl -hidden str lsc_langservers %{
	go:go-langserver
}

#Manually launch the language client binary
hook global NormalKey 0 %{ nop %sh{
   (/mnt/e/GoWorkspace/bin/kakoune-languageclient $kak_session $kak_client) > /dev/null 2>&1 < /dev/null &
}}

#Send a ping to the client that will write into the debug buffer
#Useful for debugging
hook global -group lsc NormalKey D %{ nop %sh{
   (printf "Ping\n" >> $kak_opt_lsc_pipe)
}}

#Used to cleanup the client and servers when Kakoune closes
hook global -group lsc KakEnd .* %{ nop %sh{
	(printf "KakEnd\n" >> $kak_opt_lsc_pipe)
}}

#Send textDocument/hover command
def lsc-hover %{ nop %sh{
	(printf "%s:%s:%s:textDocument/hover:%s,%s,%s\n" $kak_opt_filetype $kak_buffile $kak_timestamp $kak_cursor_line $kak_cursor_char_column) >> $kak_opt_lsc_pipe }
}

#Send textDocument/signatureHelp command
def lsc-sig-help %{ nop %sh{
	(printf "%s:%s:%s:textDocument/signatureHelp:%s,%s,%s\n" $kak_opt_filetype $kak_buffile $kak_timestamp $kak_cursor_line $kak_cursor_char_column) >> $kak_opt_lsc_pipe }
}

#Manual bindings for commands
map -docstring %{Hover help} global user h ':lsc-hover<ret>'
map -docstring %{Signature help} global user b ':lsc-sig-help<ret>'
