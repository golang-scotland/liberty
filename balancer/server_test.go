package balancer

import "testing"

func TestAcmeDirectory(t *testing.T) {
	client := newAcmeClient()
	if client.DirectoryURL != letsEncryptSandboxUrl {
		t.Errorf("sandbox url for ACME not set")
	}
}
