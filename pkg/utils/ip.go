package utils

import (
	"net"
	"strings"
)

func CheckAccess(ip string, allow, deny string) bool {
	clientIP := net.ParseIP(ip)
	if clientIP == nil {
		return false
	}

	// Deny has priority? Usually Deny first, then Allow. Or Allow first?
	// Common logic:
	// If Deny matches -> Deny.
	// If Allow matches -> Allow.
	// Default?
	// User didn't specify default.
	// Usually "HostAllow" implies default deny?
	// Or "HostDeny" implies default allow?
	// If both present?
	// Let's follow: Deny list checked. If match -> false.
	// Then Allow list checked. If match -> true.
	// If Allow list is empty -> Allow (unless Deny matched).
	// If Allow list is NOT empty -> Default Deny.

	// Parse lists
	denies := parseList(deny)
	allows := parseList(allow)

	for _, d := range denies {
		if matchIP(clientIP, d) {
			return false
		}
	}

	if len(allows) == 0 {
		return true
	}

	for _, a := range allows {
		if matchIP(clientIP, a) {
			return true
		}
	}

	return false
}

func parseList(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var res []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			res = append(res, p)
		}
	}
	return res
}

func matchIP(ip net.IP, pattern string) bool {
	// Try CIDR
	_, ipnet, err := net.ParseCIDR(pattern)
	if err == nil {
		return ipnet.Contains(ip)
	}
	// Try exact match
	pIP := net.ParseIP(pattern)
	if pIP != nil {
		return pIP.Equal(ip)
	}
	return false
}
