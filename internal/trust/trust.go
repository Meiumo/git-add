// Package trust exports the OS certificate store into a PEM bundle.
//
// Go's crypto/x509 reads the system roots on every platform, so a corporate
// root installed by MDM normally just works. The bundle still matters for two
// cases: an explicit CA file handed over by IT, and Windows, where the machine
// store is not always consulted for a root that arrived through group policy.
package trust

import (
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// StoreName describes where roots come from on this platform.
func StoreName() string {
	switch runtime.GOOS {
	case "darwin":
		return "macOS keychain"
	case "windows":
		return "Windows certificate store"
	default:
		return "system CA directory"
	}
}

// Pool returns the certificate pool to use: system roots plus the extra
// bundle when one is configured. A missing or unreadable file is reported so
// the caller can fall back rather than silently trusting less.
func Pool(caFile string) (*x509.CertPool, error) {
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	if caFile == "" {
		return pool, nil
	}
	pem, err := os.ReadFile(caFile)
	if err != nil {
		return pool, fmt.Errorf("reading %s: %w", caFile, err)
	}
	if !pool.AppendCertsFromPEM(pem) {
		return pool, fmt.Errorf("no certificates found in %s", caFile)
	}
	return pool, nil
}

// Export writes the OS trust store to dest and returns how many certificates
// it holds. Used by `git-add --fix-ca` when a corporate root needs pinning.
func Export(dest string) (int, error) {
	pem, count, err := collect()
	if err != nil {
		return 0, err
	}
	if count == 0 {
		return 0, fmt.Errorf("no certificates found in the %s", StoreName())
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return 0, err
	}
	if err := os.WriteFile(dest, pem, 0o644); err != nil {
		return 0, err
	}
	return count, nil
}
