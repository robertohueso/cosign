//
// Copyright 2021 The Sigstore Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pivkey

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"

	"github.com/go-piv/piv-go/piv"
	"github.com/sigstore/cosign/pkg/cosign"
	"github.com/sigstore/sigstore/pkg/signature"
	"golang.org/x/term"
)

func GetKey() (*piv.YubiKey, error) {
	cards, err := piv.Cards()
	if err != nil {
		return nil, err
	}
	if len(cards) == 0 {
		return nil, errors.New("no cards found")
	}
	if len(cards) > 1 {
		return nil, fmt.Errorf("found %d cards, please attach only one", len(cards))
	}
	yk, err := piv.Open(cards[0])
	if err != nil {
		return nil, err
	}
	return yk, nil
}

func getPin() (string, error) {
	fmt.Fprint(os.Stderr, "Enter PIN for security key: ")
	b, err := term.ReadPassword(0)
	if err != nil {
		return "", err
	}
	fmt.Fprintln(os.Stderr, "\nPlease tap security key...")
	return string(b), err
}

func NewPublicKeyProvider() (cosign.PublicKey, error) {
	pk, err := GetKey()
	if err != nil {
		return nil, err
	}
	cert, err := pk.Attest(piv.SlotSignature)
	if err != nil {
		return nil, err
	}
	return &PIVSigner{
		Pub: cert.PublicKey,
		ECDSAVerifier: signature.ECDSAVerifier{
			Key:     cert.PublicKey.(*ecdsa.PublicKey),
			HashAlg: crypto.SHA256,
		},
	}, nil
}

func NewSigner() (signature.Signer, error) {
	pk, err := GetKey()
	if err != nil {
		return nil, err
	}
	cert, err := pk.Attest(piv.SlotSignature)
	if err != nil {
		return nil, err
	}

	auth := piv.KeyAuth{
		PINPrompt: getPin,
	}
	privKey, err := pk.PrivateKey(piv.SlotSignature, cert.PublicKey, auth)
	if err != nil {
		return nil, err
	}
	return &PIVSigner{
		Priv: privKey,
		Pub:  cert.PublicKey,
		ECDSAVerifier: signature.ECDSAVerifier{
			Key:     cert.PublicKey.(*ecdsa.PublicKey),
			HashAlg: crypto.SHA256,
		},
	}, nil
}

type PIVSigner struct {
	Priv crypto.PrivateKey
	Pub  crypto.PrivateKey
	signature.ECDSAVerifier
}

func (ps *PIVSigner) Sign(ctx context.Context, rawPayload []byte) ([]byte, []byte, error) {
	signer := ps.Priv.(crypto.Signer)
	h := sha256.Sum256(rawPayload)
	sig, err := signer.Sign(rand.Reader, h[:], crypto.SHA256)
	if err != nil {
		return nil, nil, err
	}
	return sig, h[:], err
}

func (ps *PIVSigner) PublicKey(context.Context) (crypto.PublicKey, error) {
	return ps.Pub, nil
}

var _ signature.Signer = &PIVSigner{}
