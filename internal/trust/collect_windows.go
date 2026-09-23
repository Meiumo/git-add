//go:build windows

package trust

import (
	"bytes"
	"encoding/pem"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Both stores matter: ROOT holds the self-signed anchors, CA holds the
// intermediates a corporate PKI issues from.
var winStores = []string{"ROOT", "CA"}

func collect() ([]byte, int, error) {
	var buf bytes.Buffer
	count := 0

	for _, name := range winStores {
		ptr, err := syscall.UTF16PtrFromString(name)
		if err != nil {
			continue
		}
		store, err := windows.CertOpenSystemStore(0, ptr)
		if err != nil {
			continue
		}

		var ctx *windows.CertContext
		for {
			ctx, err = windows.CertEnumCertificatesInStore(store, ctx)
			if err != nil || ctx == nil {
				break
			}
			der := unsafe.Slice(ctx.EncodedCert, ctx.Length)
			cp := make([]byte, len(der))
			copy(cp, der)
			if err := pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE", Bytes: cp}); err == nil {
				count++
			}
		}
		windows.CertCloseStore(store, 0)
	}
	return buf.Bytes(), count, nil
}
