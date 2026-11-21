package commands

// processAlive is not needed on Windows, where the tunnel tests skip: their
// fake cloudflared is a shell script.
func processAlive(int) bool { return false }
