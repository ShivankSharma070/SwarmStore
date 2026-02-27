package main

import (
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

// CASPathTransformFunc uses a key to generate a path which can be used to
// store a file on disk. A hash values is generated for a key, which is then
// fragmented in fixed blockSize to get a CAS Path
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

// PathTransformFunc is used to generate path for storing a file by using
// generating hash value of a key
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
	// Root is the directory name of the root, containing all
	// the files/directories of the system
	root string
	PathTransformFunc
}

type Store struct {
	StoreOpts
}

// NewStore creates a *Store object using the opts provided.
// If PathTransformFunc is not provided in the opts, a DefaultPathTransformFunc
// will be used
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

// Clear will remove all the storage file on a server by removing the root
// directory used for storing files
func (s *Store) Clear() error {
	defer func() {
		log.Println("Removed all files")
	}()
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

// Write is used to store data on disk, the path of the stored file is
// determined by the key provided. Key should be unique for each file to be
// stored
func (s *Store) Write(key string, r io.Reader) (int64, error) {
	return s.writeStream(key, r)
}

// Read is used to retrieve file content from the disk
func (s *Store) Read(key string) (int64,io.Reader, error) {
	return s.readStream(key)
	// n, f, err := s.readStream(key)
	// if err != nil {
	// 	return 0,nil, err
	// }
	//
	// defer f.Close()
	//
	// buf := new(bytes.Buffer)
	// _, err = io.Copy(buf, f)
	// return n, buf, err
}

func (s *Store) readStream(key string) (int64, io.ReadCloser, error) {
	pathKey := s.PathTransformFunc(key)
	FullNameWithRoot := fmt.Sprintf("%s/%s", s.root, pathKey.FullPath())


	file, err :=os.Open(FullNameWithRoot)
	if err != nil {
		return 0, nil, err
	}

	fileStat, err := file.Stat()
	if err != nil {
		return 0, nil, err
	}
	return fileStat.Size(), file, nil
}

func (s *Store) writeDecrypt(encKey []byte, key string, r io.Reader) (int64, error) {
	f, err := s.openFileForWriting(key)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	n, err:= copyDecrypt(encKey, r, f)
	return int64(n), err 
}

func (s *Store) writeStream(key string, r io.Reader) (int64, error) {
	f, err := s.openFileForWriting(key)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return io.Copy(f, r)
}

func (s *Store) openFileForWriting(key string) (*os.File, error) {
	pathKey := s.PathTransformFunc(key)
	pathNameWithRoot := fmt.Sprintf("%s/%s", s.root, pathKey.Pathname)

	if err := os.MkdirAll(pathNameWithRoot, os.ModePerm); err != nil {
		return nil, err
	}

	FullNameWithRoot := fmt.Sprintf("%s/%s", pathNameWithRoot, pathKey.Filename)

	return os.Create(FullNameWithRoot)
} 
