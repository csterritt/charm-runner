package debug

import (
	"net"
)

// Application constants, defining host, port, and protocol.
const (
	debugListenerHost = "localhost"
	debugListenerPort = "21212"
	debugListenerType = "tcp"
)

func DumpStringToDebugListener(output string) {
	conn, err := net.Dial(debugListenerType, debugListenerHost+":"+debugListenerPort)
	if err != nil {
		return
	}
	defer conn.Close()

	// Send to socket connection.
	_, _ = conn.Write([]byte(output + "\n"))
}
