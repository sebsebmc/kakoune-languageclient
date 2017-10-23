echo -debug "Loaded lsc.kakrc"

hook global NormalKey 0 %{ nop %sh{
   ($HOME/go/bin/kakoune-languageclient/client $kak_session $kak_client) > /dev/null 2>&1 < /dev/null &
}}

hook global NormalKey D %{ nop %sh{
   (printf "Ping\n" >> $kak_opt_lsc_pipe)
}}
