package main

import (
	"crypto/rand"
	"crypto/rsa"
	// "crypto/sha1"
	"crypto/x509"
	"encoding/pem"
	"io/ioutil"
	

	"log"
	"os"
	// "fmt"
)

func bootstrapCrypto() (priv *rsa.PrivateKey, err error) {
	
	key, err := ioutil.ReadFile("pem.pem")
	
	if err != nil {
		priv, err = keygen() 
		if err != nil {
			return
		}
	} else {
		// Extract the PEM-encoded data block
		block, _ := pem.Decode(key)
		if block == nil {
			log.Fatalf("bad key data: %s", "not PEM-encoded")
		}
		if got, want := block.Type, "RSA PRIVATE KEY"; got != want {
			log.Fatalf("unknown key type %q, want %q", got, want)
		}

		// Decode the RSA private key
		priv, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			log.Fatalf("bad private key: %s", err)
		}
	}

	return
}

func keygen() (priv *rsa.PrivateKey, err error) {
	priv, err = rsa.GenerateKey(rand.Reader, 2048) // by default
	if err != nil {
		log.Fatalf("failed to generate private key: %s", err)
		return
	}

	pemFile, err := os.Create("pem.pem")
	if err != nil {
		log.Fatalf("Failed to open pem.poem for writing: %s", err)
		return
	}

	pem.Encode(pemFile, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	pemFile.Close()
	log.Println("Written pem.pem")
	return
}