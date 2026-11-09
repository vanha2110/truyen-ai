package updater

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"
)

func TestRejectTamperedManifest(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	r := Release{Version: "0.2.0", URL: "https://example.test/setup.exe", SHA256: strings.Repeat("ab", 32)}
	r.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(private, []byte(r.Version+"\n"+r.URL+"\n"+r.SHA256)))
	key := base64.StdEncoding.EncodeToString(public)
	if err = Verify(r, key); err != nil {
		t.Fatal(err)
	}
	r.URL = "https://example.test/tampered.exe"
	if Verify(r, key) == nil {
		t.Fatal("tampered manifest accepted")
	}
	r.Version = "../../evil"
	if Verify(r, key) == nil {
		t.Fatal("invalid filename accepted")
	}
}

func TestVersionOrder(t *testing.T) {
	for _, candidate := range []string{"0.0.9", "0.1.0", "bad"} {
		if Newer(candidate, "0.1.0") {
			t.Fatalf("unsafe upgrade %s", candidate)
		}
	}
	if !Newer("0.10.0", "0.9.0") {
		t.Fatal("lexical version comparison")
	}
}
