package crypto

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	"github.com/hiddeco/sshsig"
	"golang.org/x/crypto/ssh"
)

func VerifySignature(pubKey, signature, payload []byte) (error, bool) {
	pub, _, _, _, err := ssh.ParseAuthorizedKey(pubKey)
	if err != nil {
		return fmt.Errorf("failed to parse public key: %w", err), false
	}

	sig, err := sshsig.Unarmor(signature)
	if err != nil {
		return fmt.Errorf("failed to parse signature: %w", err), false
	}

	buf := bytes.NewBuffer(payload)
	// we use sha-512 because ed25519 keys require it internally; rsa keys support
	// multiple algorithms but sha-512 is most secure, and git's ssh signing defaults
	// to sha-512 for all key types anyway.
	err = sshsig.Verify(buf, sig, pub, sshsig.HashSHA512, "git")

	return err, err == nil
}

// SSHFingerprint computes the fingerprint of the supplied ssh pubkey.
func SSHFingerprint(pubKey string) (string, error) {
	pk, _, _, _, err := ssh.ParseAuthorizedKey([]byte(pubKey))
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(pk.Marshal())
	return "SHA256:" + base64.StdEncoding.EncodeToString(hash[:]), nil
}
