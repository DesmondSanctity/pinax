# VHS tape — renders assets/demo.gif
# Render with: vhs assets/demo.tape

Output assets/demo.gif

Set Shell "zsh"
Set FontSize 20
Set Width 1600
Set Height 900
Set Padding 30
Set Framerate 50
Set TypingSpeed 45ms
Set PlaybackSpeed 1.0
Set Theme { "name": "Pinax", "background": "#0d1117", "foreground": "#e6edf3", "cursor": "#7dd3fc", "black": "#0d1117", "red": "#f87171", "green": "#5eead4", "yellow": "#fbbf24", "blue": "#7dd3fc", "magenta": "#c084fc", "cyan": "#67e8f9", "white": "#e6edf3", "brightBlack": "#475569", "brightRed": "#fca5a5", "brightGreen": "#6ee7b7", "brightYellow": "#fcd34d", "brightBlue": "#93c5fd", "brightMagenta": "#d8b4fe", "brightCyan": "#a5f3fc", "brightWhite": "#f8fafc", "selection": "#1f6feb" }

Hide
Type "clear" Enter
Sleep 500ms
Show

# Step 1 — show the banner + usage
Type "pinax"
Sleep 400ms
Enter
Sleep 3s

Hide
Type "clear" Enter
Sleep 300ms
Show

# Step 2 — list the docs sites already indexed
Type "pinax list"
Sleep 400ms
Enter
Sleep 2.5s

# Step 3 — boot the MCP server over HTTP with the built-in log UI
Type "pinax serve docs-convex-dev --http --port 8423"
Sleep 400ms
Enter
Sleep 4s

# Clean shutdown
Ctrl+C
Sleep 1.5s
