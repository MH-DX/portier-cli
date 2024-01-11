package encryption

import "github.com/marinator86/portier-cli/internal/portier/relay/messages"

// Cipher is the cipher type and can be AES-256-GCM.
type Cipher string

// Curve is the curve type and can be P256.
type Curve string

type encryption struct {
	// PublicKey is the public key
	localPublicKey string

	// PrivateKey is the private key
	localPrivateKey string

	// PeerPublicKey is the peer public key
	peerPublicKey string

	// sessionKey is the session key
	// Cipher is the cipher that is used to encrypt the data
	Cipher Cipher

	// Curve is the canonical name of the curve that is used to generate the keys
	Curve Curve
}

type Encryption interface {
	// Encrypt encrypts the data
	Encrypt(header messages.MessageHeader, data []byte) ([]byte, error)

	// Decrypt decrypts the data
	Decrypt(header messages.MessageHeader, data []byte) ([]byte, error)
}

func (e *encryption) Encrypt(_ messages.MessageHeader, data []byte) ([]byte, error) {
	return data, nil
}

func (e *encryption) Decrypt(_ messages.MessageHeader, data []byte) ([]byte, error) {
	return data, nil
}

// NewEncryption creates a new encryption.
func NewEncryption(localPublicKey string, localPrivateKey string, peerPublicKey string, cipher Cipher, curve Curve) Encryption {
	return &encryption{
		localPublicKey:  localPublicKey,
		localPrivateKey: localPrivateKey,
		peerPublicKey:   peerPublicKey,
		Cipher:          cipher,
		Curve:           curve,
	}
}
