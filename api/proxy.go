package api

import "fmt"

// IsValidPort reports whether p is a valid TCP/UDP port number (1-65535).
func IsValidPort(p int) bool {
	return p >= 1 && p <= 65535
}

// ProxyURL builds the subdomain-based proxy URL for a forwarded port.
// Format: http://{projectId}-{agentType}-{port}.localhost:{serverPort}/
func ProxyURL(serverPort, projectID, agentType string, port int) string {
	return fmt.Sprintf("http://%s-%s-%d.localhost:%s/", projectID, agentType, port, serverPort)
}
