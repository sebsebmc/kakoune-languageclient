echo -debug "Loading lsc.kakrc"

hook global NormalKey 0 %{ nop %sh{
   (/mnt/e/GoWorkspace/bin/kakoune-languageclient $kak_session $kak_client) > /dev/null 2>&1 < /dev/null &
}}

hook global -group lsp NormalKey D %{ nop %sh{
   (printf "Ping\n" >> $kak_opt_lsc_pipe)
}}

hook global -group lsp KakEnd .* %{ nop %sh{
	(printf "KakEnd\n" >> $kak_opt_lsc_pipe)
}}

def lsp-hover %{ nop %sh{
	(printf "%s:textDocument/hover:%s,%s,%s\n" $kak_opt_filetype $kak_buffile $kak_cursor_line $kak_cursor_char_column) >> $kak_opt_lsc_pipe }
}

def lsp-sig-help %{ nop %sh{
	(printf "%s:textDocument/signatureHelp:%s,%s,%s\n" $kak_opt_filetype $kak_buffile $kak_cursor_line $kak_cursor_char_column) >> $kak_opt_lsc_pipe }
}

map -docstring %{Hover help} global user h ':lsp-hover<ret>'
map -docstring %{Signature help} global user b ':lsp-sig-help<ret>'
