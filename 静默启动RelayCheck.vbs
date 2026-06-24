Set shell = CreateObject("Wscript.Shell")
shell.CurrentDirectory = "E:\zidqiandao\relaycheck-desktop"
shell.Environment("PROCESS")("RELAYCHECK_PORT") = "3001"
shell.Environment("PROCESS")("RELAYCHECK_NO_OPEN") = "1"
shell.Run """E:\zidqiandao\relaycheck-desktop\dist\relaycheck.exe""", 0, False
