# vhs tape for demo.gif — run via demo/record.sh (starts the demo ssh server
# and stages /tmp/portside-demo first)
Output demo/demo.gif

Set FontSize 15
Set Width 760
Set Height 600
Set Padding 12
Set Theme "Catppuccin Mocha"
Set TypingSpeed 60ms

Hide
Type "export HOME=/tmp/portside-demo/home; clear"
Enter
Sleep 500ms
Show

Type "portside --host prod-server"
Sleep 1s
Enter
Sleep 2.5s

# expand app/ (cursor starts on it)
Enter
Sleep 1.2s
# down to src/ and expand it
Down
Sleep 350ms
Down
Sleep 350ms
Down
Sleep 350ms
Enter
Sleep 1.2s
# select api.go
Down
Sleep 600ms

# download it to the local machine
Type "d"
Sleep 1.5s
Enter
Sleep 2s

# send the path to the claude pane / clipboard
Type "c"
Sleep 2s

# ports view: forward a port
Ctrl+P
Sleep 1.5s
Type "a"
Sleep 800ms
Type "8080"
Sleep 500ms
Tab
Sleep 400ms
Type "8080"
Sleep 500ms
Enter
Sleep 2.5s

# back to the explorer
Ctrl+P
Sleep 2s

Type "q"
Sleep 1s
