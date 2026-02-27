package main

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCopyEncrypt(t *testing.T) {
	payload := "Foo not bar"
	src := bytes.NewReader([]byte(payload))
	dest := new(bytes.Buffer)
	key := newEncryptionKey()
	nw, err := copyEncrypt(key, src, dest)
	assert.Nil(t, err)
	assert.Equal(t,nw, 16+len(payload))

	fmt.Println(dest.String())

	buf := new(bytes.Buffer)
	nw, err = copyDecrypt(key, dest, buf)
	assert.Nil(t, err)

	fmt.Println(buf.String())
	assert.Equal(t, buf.String(), payload)
}
