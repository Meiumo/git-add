//go:build !darwin && !windows

package trust

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
)

var (
	linuxBundles = []string{
		"/etc/ssl/certs/ca-certificates.crt",
		"/etc/pki/tls/certs/ca-bundle.crt",
		"/etc/ssl/ca-bundle.pem",
	}
	linuxAnchorDirs = []string{
		"/usr/local/share/ca-certificates",
		"/etc/pki/ca-trust/source/anchors",
	}
)

func collect() ([]byte, int, error) {
	var buf bytes.Buffer
	count := 0

	for _, path := range linuxBundles {
		body, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		buf.Write(body)
		buf.WriteByte('\n')
		count += bytes.Count(body, []byte("BEGIN CERTIFICATE"))
		break
	}

	for _, dir := range linuxAnchorDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if ext != ".crt" && ext != ".pem" && ext != ".cer" {
				continue
			}
			body, err := os.ReadFile(filepath.Join(dir, entry.Name()))
			if err != nil || !bytes.Contains(body, []byte("BEGIN CERTIFICATE")) {
				continue
			}
			buf.Write(body)
			buf.WriteByte('\n')
			count += bytes.Count(body, []byte("BEGIN CERTIFICATE"))
		}
	}
	return buf.Bytes(), count, nil
}
