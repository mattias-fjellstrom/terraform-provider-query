# VHS tape for tpq — https://github.com/charmbracelet/vhs
#
# Render with:
#   vhs demo.tape          # produces demo.gif (and demo.mp4 / demo.webm if configured below)
#
# Tip: keep the recording short (< 20s) and the dimensions modest so the
# resulting GIF stays small. For the README, prefer uploading the .mp4 via a
# GitHub issue/PR comment and embedding the user-attachments URL.

Output demo.gif
Output demo.mp4
Output demo.webm

# Terminal appearance
Set Shell        "bash"
Set FontSize     14
Set Width        1000
Set Height       600
Set Padding      20
Set Theme        "Catppuccin Mocha"
Set TypingSpeed  60ms
Set PlaybackSpeed 1.0
Set Margin 20
Set MarginFill "#674EFF"
Set BorderRadius 10

# Make sure we use the freshly built binary from this repo
Hide
Type "go build -o tpq . && export PATH=$PWD:$PATH && clear"
Enter
Sleep 3s
Show

# Launch the TUI
Type "tpq"
Sleep 500ms
Enter
Sleep 3000ms

# Filter the provider list
Type "azurerm"
Sleep 2000ms

# Drop into the versions screen for the highlighted provider
Enter
Wait+Screen /published/

# Select the latest version and open release notes
Hide
Sleep 2000ms
Enter
Wait+Screen /Release notes/
Sleep 2000ms
Escape
Show

Sleep 2000ms
Down 3
Sleep 500ms
Up 3
Sleep 500ms
Enter
Wait+Screen /Release notes/
Sleep 2000ms
Down 10

# Back out to versions, then to browse, then quit
Escape
Sleep 1000ms
Escape
Sleep 1000ms
Ctrl+C
Sleep 1000ms
