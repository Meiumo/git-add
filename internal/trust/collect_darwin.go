//go:build darwin

package trust

import (
	"bytes"
	"os"
	"os/exec"
)

var macKeychains = []string{
	"/System/Library/Keychains/SystemRootCertificates.keychain",
	"/Library/Keychains/System.keychain",
}

func collect() ([]byte, int, error) {
	var buf bytes.Buffer
	count := 0
	for _, kc := range macKeychains {
		if _, err := os.Stat(kc); err != nil {
			continue
		}
		out, err := exec.Command("security", "find-certificate", "-a", "-p", kc).Output()
		if err != nil {
			continue
		}
		buf.Write(out)
		buf.WriteByte('\n')
		count += bytes.Count(out, []byte("BEGIN CERTIFICATE"))
	}
	return buf.Bytes(), count, nil
}
