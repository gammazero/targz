package targz_test

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/gammazero/targz"
	"github.com/stretchr/testify/require"
)

func TestArchiveDir(t *testing.T) {
	const (
		archName    = "test.tar.gz"
		srcName     = "src/"
		subDirName  = "sub/"
		subFileName = "bork.txt"
	)

	dummyData := []byte("hello world")
	dataSize := int64(len(dummyData))

	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, srcName)
	err := os.Mkdir(srcDir, 0750)
	require.NoError(t, err)

	// Write some files to the source directory. These are in alpha-order
	// because files in archive will be in alpha-order.
	files := []string{"bar.txt", "baz.txt", "foo.txt"}
	paths := make([]string, len(files))
	for i := range files {
		name := filepath.Join(srcDir, files[i])
		f, err := os.Create(name)
		require.NoError(t, err)
		f.Write(dummyData)
		f.Close()
		paths[i] = name
	}

	subDir := filepath.Join(srcDir, subDirName)
	err = os.Mkdir(subDir, 0750)
	require.NoError(t, err)
	f, err := os.Create(filepath.Join(subDir, subFileName))
	require.NoError(t, err)
	f.Write(dummyData)
	f.Close()

	tarPath := filepath.Join(tmpDir, archName)
	err = targz.Create(srcDir, tarPath)
	require.NoError(t, err)

	fi, err := os.Stat(tarPath)
	require.NoError(t, err)
	require.NotZero(t, fi.Size())

	f, err = os.Open(tarPath)
	require.NoError(t, err)
	defer f.Close()

	// Read the archive.
	gzr, err := gzip.NewReader(f)
	require.NoError(t, err)
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	hdr, err := tr.Next()
	require.NoError(t, err)
	require.Equal(t, srcName, hdr.Name)

	// Check that all files are present.
	for _, fileName := range files {
		hdr, err = tr.Next()
		require.NoError(t, err)
		require.Equal(t, path.Join(srcName, fileName), hdr.Name)
		require.Equal(t, dataSize, hdr.Size)
	}

	// Check that subdirectory is present.
	hdr, err = tr.Next()
	require.NoError(t, err)
	require.Equal(t, path.Join(srcName, subDirName)+"/", hdr.Name)

	// Check that file in subdirectory is present.
	hdr, err = tr.Next()
	require.NoError(t, err)
	require.Equal(t, path.Join(srcName, subDirName, subFileName), hdr.Name)

	// Check that no additional files are in archive.
	hdr, err = tr.Next()
	require.ErrorIs(t, err, io.EOF, "archive has wrong number of items")

	// Remove the source directory to prepare for extraction test.
	require.NoError(t, os.RemoveAll(srcDir))

	// Extract the archive.
	err = targz.Extract(tarPath, tmpDir)
	require.NoError(t, err)

	// Verify directory contents.
	dirEnts, err := os.ReadDir(srcDir)
	require.NoError(t, err)
	subDir = ""
	subName := filepath.Clean(subDirName)
	var fileCount int
	for i, de := range dirEnts {
		fname := de.Name()
		if fname == subName {
			subDir = filepath.Join(srcDir, subName)
			break
		}
		require.Equal(t, files[i], fname)
		fi, err := de.Info()
		require.NoError(t, err)
		require.Equal(t, dataSize, fi.Size())
		fileCount++
	}
	require.Equal(t, 3, fileCount)
	require.Equal(t, filepath.Join(srcDir, subName), subDir)

	// Verify subdirectory extracted.
	require.NotEmpty(t, subDir)
	dirEnts, err = os.ReadDir(subDir)
	require.NoError(t, err)
	for i, de := range dirEnts {
		require.Zero(t, i, "subdir has too many files")
		fname := de.Name()
		require.Equal(t, subFileName, fname)
		fi, err := de.Info()
		require.NoError(t, err)
		require.Equal(t, dataSize, fi.Size())
	}
}

func TestIgnore(t *testing.T) {
	const archName = "test.tar.gz"
	const srcName = "src/"

	dummyData := []byte("hello world")
	dataSize := int64(len(dummyData))

	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, srcName)
	err := os.Mkdir(srcDir, 0750)
	require.NoError(t, err)

	// Write some files to the source directory. These are in alpha-order
	// because files in archive will be in alpha-order.
	files := []string{"bar.txt", "baz.txt", "foo.txt"}
	paths := make([]string, len(files))
	for i := range files {
		name := filepath.Join(srcDir, files[i])
		f, err := os.Create(name)
		require.NoError(t, err)
		f.Write(dummyData)
		f.Close()
		paths[i] = name
	}

	tarPath := filepath.Join(tmpDir, archName)
	err = targz.Create(srcDir, tarPath, targz.WithIgnore("baz.txt"))
	require.NoError(t, err)

	fi, err := os.Stat(tarPath)
	require.NoError(t, err)
	require.NotZero(t, fi.Size())

	f, err := os.Open(tarPath)
	require.NoError(t, err)
	defer f.Close()

	// Read the subarchive and make sure it has each of the files
	gzr, err := gzip.NewReader(f)
	require.NoError(t, err)
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	hdr, err := tr.Next()
	require.NoError(t, err)
	require.Equal(t, srcName, hdr.Name)

	files[1] = files[2]
	files = files[:len(files)-1]

	var i int
	for hdr, err = tr.Next(); err == nil; hdr, err = tr.Next() {
		require.Equal(t, path.Join(srcName, files[i]), hdr.Name)
		require.Equal(t, dataSize, hdr.Size)
		i++
	}
	require.ErrorIs(t, err, io.EOF)
	require.Equal(t, len(files), i, "archive has wrong number of files")
}
