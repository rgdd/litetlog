package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/pem"
	"flag"
	"fmt"
	"log"

	"github.com/mikesmitty/edkey"
	"golang.org/x/crypto/hkdf"
	"golang.org/x/crypto/ssh"
	"golang.org/x/mod/sumdb/tlog"
	sigsum "sigsum.org/sigsum-go/pkg/crypto"
	"sigsum.org/sigsum-go/pkg/merkle"
)

var seedFlag = flag.String("seed", "", "hex-encoded seed")

func main() {
	flag.Parse()
	var seed []byte
	if *seedFlag == "" {
		seed = make([]byte, 32)
		if _, err := rand.Read(seed); err != nil {
			log.Fatal(err)
		}
	} else {
		seed = make([]byte, hex.DecodedLen(len(*seedFlag)))
		if _, err := hex.Decode(seed, []byte(*seedFlag)); err != nil {
			log.Fatal(err)
		}
	}
	fmt.Printf("- seed: %x\n", seed)
	h := hkdf.New(sha256.New, seed, []byte("litewitness gentest"), nil)

	var privateKey sigsum.PrivateKey
	h.Read(privateKey[:])
	fmt.Printf("- log private key: %x\n", privateKey)
	s := sigsum.NewEd25519Signer(&privateKey)
	publicKey := s.Public()
	fmt.Printf("- log public key: %x\n", publicKey)
	keyHash := sigsum.HashBytes(publicKey[:])
	fmt.Printf("- log key hash: %x\n", keyHash)
	origin := fmt.Sprintf("sigsum.org/v1/tree/%x", keyHash)

	witKey := ed25519.PrivateKey(make([]byte, ed25519.PrivateKeySize))
	h.Read(witKey)
	ss, err := ssh.NewSignerFromSigner(witKey)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("- witness key fingerprint: %s\n", ssh.FingerprintSHA256(ss.PublicKey()))
	fmt.Printf("- witness key: %x\n", witKey)
	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "OPENSSH PRIVATE KEY",
		Bytes: edkey.MarshalED25519PrivateKey(witKey),
	})
	fmt.Printf("- witness key:\n%s", pemBytes)

	tree := merkle.NewTree()
	rootHash := func() {
		fmt.Printf("- root hash (size %d): %x\n", tree.Size(), tree.GetRootHash())
	}
	addLeaf := func(leaf sigsum.Hash) {
		if !tree.AddLeafHash(&leaf) {
			panic("duplicate")
		}
		fmt.Printf("- leaf[%d] hash: %x\n", tree.Size(), leaf)
	}
	signTreeHead := func() {
		checkpoint := fmt.Sprintf("%s\n%d\n%s\n", origin, tree.Size(), tlog.Hash(tree.GetRootHash()))
		fmt.Printf("- checkpoint:\n%s", checkpoint)

		signature, err := s.Sign([]byte(checkpoint))
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("- signature: %x\n", signature)
	}
	consistencyProof := func(oldSize uint64) {
		proof, err := tree.ProveConsistency(oldSize, tree.Size())
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("- consistency proof from size %d:\n", oldSize)
		for _, p := range proof {
			fmt.Printf("%x\n", p)
		}
	}

	addLeaf(sigsum.Hash{42, 0})
	rootHash()
	signTreeHead()

	addLeaf(sigsum.Hash{42, 1})
	addLeaf(sigsum.Hash{42, 2})
	rootHash()
	consistencyProof(1)
	signTreeHead()

	addLeaf(sigsum.Hash{42, 3})
	addLeaf(sigsum.Hash{42, 4})
	rootHash()
	consistencyProof(1)
	consistencyProof(3)
	signTreeHead()
}
