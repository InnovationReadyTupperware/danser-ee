package main

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) != 3 {
		log.Fatalf("usage: %s DST SRC", filepath.Base(os.Args[0]))
	}

	if err := pack(os.Args[1], os.Args[2]); err != nil {
		panic(err)
	}
}

// pack archives the directory tree rooted at src into a new zip file at
// dst, preserving permission bits so executables stay executable.
func pack(dst, src string) error {
	dstFile, err := filepath.Abs(dst)
	if err != nil {
		return fmt.Errorf("resolve destination path: %w", err)
	}

	file, err := os.Create(dstFile)
	if err != nil {
		return fmt.Errorf("create archive: %w", err)
	}

	srcPath, err := filepath.Abs(src)
	if err != nil {
		file.Close()
		return fmt.Errorf("resolve source path: %w", err)
	}
	srcPath += string(os.PathSeparator)

	writer := zip.NewWriter(file)

	walkErr := filepath.Walk(srcPath, func(osFilePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		return packFile(writer, srcPath, osFilePath, info)
	})
	if walkErr != nil {
		writer.Close()
		file.Close()
		return fmt.Errorf("pack %q: %w", srcPath, walkErr)
	}

	if err := writer.Close(); err != nil {
		file.Close()
		return fmt.Errorf("finalize archive: %w", err)
	}

	if err := file.Close(); err != nil {
		return fmt.Errorf("close archive: %w", err)
	}

	log.Println("Finished")
	return nil
}

// packFile stores one source file in the archive.
func packFile(writer *zip.Writer, srcPath, osFilePath string, info os.FileInfo) error {
	trunc := strings.ReplaceAll(strings.TrimPrefix(osFilePath, srcPath), "\\", "/")

	log.Println("Packing:", osFilePath)

	header, err := entryHeader(trunc, info)
	if err != nil {
		return err
	}

	fileWriter, err := writer.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("create entry: %w", err)
	}

	fileReader, err := os.Open(osFilePath)
	if err != nil {
		return fmt.Errorf("open source file: %w", err)
	}
	defer fileReader.Close()

	if _, err := io.Copy(fileWriter, fileReader); err != nil {
		return fmt.Errorf("write entry: %w", err)
	}

	return nil
}

// entryHeader builds the archive entry for one source file, carrying
// over its permission bits.
func entryHeader(name string, info os.FileInfo) (*zip.FileHeader, error) {
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return nil, fmt.Errorf("build entry header: %w", err)
	}
	header.Name = name
	header.Method = zip.Deflate
	return header, nil
}
