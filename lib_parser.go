package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/blakesmith/ar"
)

const LibraryFileExtension string = ".lib"
const NullImportDescriptor string = "__NULL_IMPORT_DESCRIPTOR"

var ErrNotDir = errors.New("not a directory")
var ErrSymbolNotFound = errors.New("failed to resolve the symbol")

type SecondLinkerMember struct {
	NumberOfMembers uint32
	Offsets         []uint32
	NumberOfSymbols uint32
	Indices         []uint16
	StringTable     bytes.Buffer
}

type SymbolResolver struct {
	slm       SecondLinkerMember
	symbolMap map[string]string
}

func NewSymbolResolver(directory string) (*SymbolResolver, error) {
	stat, err := os.Stat(directory)
	if err != nil {
		return nil, err
	}
	if !stat.IsDir() {
		return nil, ErrNotDir
	}

	symRes := &SymbolResolver{
		symbolMap: make(map[string]string),
	}

	fileinfos, err := ioutil.ReadDir(directory)
	for _, fileinfo := range fileinfos {
		if !strings.HasSuffix(strings.ToLower(fileinfo.Name()), LibraryFileExtension) {
			log.Printf("Skipping non-library file: %s\n", fileinfo.Name())
			continue
		}

		file, err := os.Open(directory + "/" + fileinfo.Name())
		if err != nil {
			log.Fatalf("Failed to open file `%s`\n", fileinfo.Name())
		}

		libraryName := strings.TrimSuffix(strings.ToLower(fileinfo.Name()), LibraryFileExtension)

		reader := ar.NewReader(file)
		// Skip to second linker member `/`
		header, err := reader.Next()
		header, err = reader.Next()
		if err != nil || header.Name != "/" {
			log.Printf("Failed to find second linker member in library `%s`\n", libraryName)
			continue
		}

		binary.Read(reader, binary.LittleEndian, &symRes.slm.NumberOfMembers)
		io.CopyN(ioutil.Discard, reader, 4*int64(symRes.slm.NumberOfMembers))
		binary.Read(reader, binary.LittleEndian, &symRes.slm.NumberOfSymbols)
		io.CopyN(ioutil.Discard, reader, 2*int64(symRes.slm.NumberOfSymbols))

		var stringTableBuffer bytes.Buffer
		io.Copy(&stringTableBuffer, reader)
		stringTable := stringTableBuffer.Bytes()
		for start := 0; start < len(stringTable); {
			end := start + bytes.IndexByte(stringTable[start:], 0)
			symbolName := string(stringTable[start:end])
			if symbolName != NullImportDescriptor {
				if previousLib, ok := symRes.symbolMap[symbolName]; ok {
					log.Printf("symbolMap: duplicate symbol %s:%s found in %s\n", previousLib, symbolName, libraryName)
				} else {
					symRes.symbolMap[symbolName] = libraryName
				}
			}
			start = end + 1

		}
		file.Close()
	}

	return symRes, nil
}

func (sr *SymbolResolver) Resolve(symbol string) (string, error) {
	if library, ok := sr.symbolMap[symbol]; ok {
		return library, nil
	}
	return "", ErrSymbolNotFound
}
