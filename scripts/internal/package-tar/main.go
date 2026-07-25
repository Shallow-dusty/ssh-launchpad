package main

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	source := flag.String("source", "", "source directory")
	output := flag.String("output", "", "output .tar.gz")
	flag.Parse()
	if *source == "" || *output == "" {
		exit(errors.New("-source and -output are required"))
	}
	if err := packageDirectory(*source, *output); err != nil {
		exit(err)
	}
}

func packageDirectory(source, output string) (returnErr error) {
	root, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return errors.New("source must be an existing directory")
	}
	rootHandle, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer rootHandle.Close()
	file, err := os.OpenFile(output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := file.Close(); returnErr == nil {
			returnErr = closeErr
		}
		if returnErr != nil {
			_ = os.Remove(output)
		}
	}()
	compressor, err := gzip.NewWriterLevel(file, gzip.BestCompression)
	if err != nil {
		return err
	}
	archive := tar.NewWriter(compressor)
	defer func() {
		if closeErr := archive.Close(); returnErr == nil {
			returnErr = closeErr
		}
		if closeErr := compressor.Close(); returnErr == nil {
			returnErr = closeErr
		}
	}()

	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symbolic links are not allowed in portable bundles: %s", path)
		}
		fileInfo, err := entry.Info()
		if err != nil {
			return err
		}
		if !fileInfo.IsDir() && !fileInfo.Mode().IsRegular() {
			return fmt.Errorf("unsupported bundle entry: %s", path)
		}
		relative, err := filepath.Rel(root, path)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return fmt.Errorf("bundle entry escapes source: %s", path)
		}
		name := filepath.ToSlash(relative)
		header, err := tar.FileInfoHeader(fileInfo, "")
		if err != nil {
			return err
		}
		header.Name = name
		header.Uid, header.Gid = 0, 0
		header.Uname, header.Gname = "root", "root"
		if fileInfo.IsDir() {
			header.Name += "/"
			header.Mode = 0o755
		} else if executableBundleFile(name) {
			header.Mode = 0o755
		} else {
			header.Mode = 0o644
		}
		if err := archive.WriteHeader(header); err != nil {
			return err
		}
		if fileInfo.IsDir() {
			return nil
		}
		input, err := rootHandle.Open(relative)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(archive, input)
		closeErr := input.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
}

func executableBundleFile(name string) bool {
	base := filepath.Base(name)
	return base == "ssh-launchpad" || strings.HasSuffix(base, ".sh") || strings.HasSuffix(base, ".command")
}

func exit(err error) {
	_, _ = fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
