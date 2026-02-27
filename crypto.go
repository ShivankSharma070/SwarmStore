package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"io"

)

func newEncryptionKey() []byte {
	keyBuf := make([]byte, 32)
	io.ReadFull(rand.Reader, keyBuf)
	return keyBuf
}

func copyDecrypt(key []byte, src io.Reader, dest io.Writer) (int, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return 0, err
	}

	iv := make([]byte, block.BlockSize())
	if _, err := src.Read(iv); err != nil {
		return 0, err
	}

	var (
		buf    = make([]byte, 32*1024)
		stream = cipher.NewCTR(block, iv)
		nw = block.BlockSize()
	)
	for {
		n, err := src.Read(buf)

		if n > 0 {
			stream.XORKeyStream(buf, buf[:n])
			nn, err := dest.Write(buf[:n]) ;
			if err != nil {
				return 0, err
			}
			nw += nn
		}

		if err == io.EOF {
			break
		}

		if err != nil {
			return 0, nil
		}

	}

	return nw,  err
}

func copyEncrypt(key []byte, src io.Reader, dest io.Writer) (int, error) {
	// Create a new block using your key
	block, err := aes.NewCipher(key)
	if err != nil {
		return 0, err
	}

	// Create a initialisation vector, which is of same size of block
	iv := make([]byte, block.BlockSize())

	// Fill iv with a random value
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return 0, err
	}

	// Write iv to destination so that it stays at the beginning of the
	// destination and can be used by decryption
	if _, err := dest.Write(iv); err != nil {
		return 0, err
	}

	var (
		buf    = make([]byte, 32*1024)
		stream = cipher.NewCTR(block, iv)
		nw = block.BlockSize()
	)

	for {
		n, err := src.Read(buf) // Read data into a bufferji
		if n > 0 {
			// This will generate a random aes encrypted data called keystream,
			// then will xor the data present in buf with the keystream and
			// store the result back into buf
			stream.XORKeyStream(buf, buf[:n])

			// Store the xor value in dest
			 nn, err := dest.Write(buf[:n]);
			 if err != nil {
				return 0, err
			}
			nw += nn
		}

		if err == io.EOF {
			break
		}

		if err != nil {
			return 0, err
		}

	}

	return nw, nil
}
