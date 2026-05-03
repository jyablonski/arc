package sysupdate

import "strings"

func parseRebootConfirmation(response string) bool {
	response = strings.TrimSpace(strings.ToLower(response))
	return response == "" || response == "y" || response == "yes"
}
