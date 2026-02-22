package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

const DefaultRootDirName = "files"
func CASPathTransformFunc(key string) PathKey {
	hash := sha1.Sum([]byte(key))
	hashStr := hex.EncodeToString(hash[:])

	blockSize := 5
	sliceLen := len(hashStr) / blockSize
	paths := make([]string, sliceLen)

	for i := 0; i < sliceLen; i++ {
		from, to := i*blockSize, (i*blockSize)+blockSize
		paths[i] = hashStr[from:to]
	}

	return PathKey{
		Pathname: strings.Join(paths, "/"),
		Filename: hashStr,
	}
}

type PathTransformFunc func(string) PathKey

type PathKey struct {
	Pathname string
	Filename string
}

func DefaultPathTransformFunc(key string) PathKey {
	return PathKey{
		Pathname: key,
		Filename: key,
	}
}

func (pk *PathKey) FullPath() string {
	return fmt.Sprintf("%s/%s", pk.Pathname, pk.Filename)
}
func (pk *PathKey) FirstPathName() string {
	paths := strings.Split(pk.Pathname, "/")
	if len(paths) == 0 {
		return ""
	}

	return paths[0]
}

type StoreOpts struct {
	// Root is the directory name of the root, containing all the files/directories
	//	of the system
	root string
	PathTransformFunc
}

type Store struct {
	StoreOpts
}

func NewStore(opts StoreOpts) *Store {
	if opts.PathTransformFunc == nil {
		opts.PathTransformFunc = DefaultPathTransformFunc
	}

	if len(opts.root) == 0 {
		opts.root = DefaultRootDirName
	}
	return &Store{
		StoreOpts: opts,
	}
}

func (s *Store) Clear() error {
	defer func() {
		log.Println("Removed all files")
	} ()
	return os.RemoveAll(s.root)
}

func (s *Store) Has(key string) bool { 
	pathKey := s.PathTransformFunc(key)
	fullNameWithRoot := fmt.Sprintf("%s/%s", s.root, pathKey.FullPath())
	_, err := os.Stat(fullNameWithRoot)
	if errors.Is(err, os.ErrNotExist) {
		return false
	}

	return true
}

func (s *Store) Delete(key string) error {
	pathKey := s.PathTransformFunc(key)
	firstPathNameWithRoot := fmt.Sprintf("%s/%s", s.root, pathKey.FirstPathName())
	defer func() {
		log.Printf("Deleted File: %s\n", pathKey.FullPath())
	}()

	return os.RemoveAll(firstPathNameWithRoot)
}

func (s *Store) Write(key string, r io.Reader) error {
	return s.writeStream(key, r)
}

func (s *Store) Read(key string) (*bytes.Buffer, error) {
	f, err := s.readStream(key)
	if err != nil {
		return nil, err
	}

	defer f.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, f)
	return buf, err
}

func (s *Store) readStream(key string) (io.ReadCloser, error) {
	pathKey := s.PathTransformFunc(key)
	FullNameWithRoot := fmt.Sprintf("%s/%s",s.root, pathKey.FullPath())
	return os.Open(FullNameWithRoot)
}

func (s *Store) writeStream(key string, r io.Reader) error {
	pathKey := s.PathTransformFunc(key)
	pathNameWithRoot := fmt.Sprintf("%s/%s", s.root, pathKey.Pathname)

	if err := os.MkdirAll(pathNameWithRoot, os.ModePerm); err != nil {
		return err
	}

	FullNameWithRoot := fmt.Sprintf("%s/%s", pathNameWithRoot, pathKey.Filename)

	f, err := os.Create(FullNameWithRoot)
	defer f.Close()
	if err != nil {
		return err
	}

	n, err := io.Copy(f, r)
	if err != nil {
		return err
	}

	log.Printf("Written (%d) bytes to disk: (%s)", n, pathKey.FullPath())

	return nil
}
