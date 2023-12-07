package targz

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

// Create creates a gzip compressed tar file containing the contents of the
// specified directory.
//
// If the directory to archive is specified by a path such as
// "/tmp/myfiles/backups/weekly", then only the "weekly" directory, and none of
// its parent path, is added to the tar archive. When extracted, a "weekly"
// directory is created with all of its archived contents.
func Create(dir, tarPath string, options ...Option) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	dir = filepath.Clean(dir)
	if dir == "" || dir == "." || dir == cwd {
		return errors.New("cannot archive current directory")
	}
	tarfile, err := os.Create(tarPath)
	if err != nil {
		return err
	}
	if err = CreateWriter(dir, tarfile, options...); err != nil {
		tarfile.Close()
		return err
	}
	// Close tar file.
	return tarfile.Close()
}

// Create writes a gzip compressed tar file to an io.Writer. The tar file
// contains the contents of the specified directory.
func CreateWriter(dir string, w io.Writer, options ...Option) error {
	opts := getOpts(options)

	wr := bufio.NewWriter(w)

	// gzip writer writes to buffer.
	gzw := gzip.NewWriter(wr)
	defer gzw.Close()
	// tar writer writes to gzip.
	tw := tar.NewWriter(gzw)
	defer tw.Close()

	err := tarAddDir(dir, opts.ignores, tw)
	if err != nil {
		return err
	}

	// Close tar writer; flush tar data to gzip writer
	if err = tw.Close(); err != nil {
		return err
	}
	// Close gzip writer; finish writing gzip data to buffer.
	if err = gzw.Close(); err != nil {
		return err
	}
	// Flush buffered data to writer.
	return wr.Flush()
}

// tarAddDir recursively writes all files and subdirectories to the tar writer.
func tarAddDir(dir string, ignores []string, tw *tar.Writer) error {
	dir = strings.TrimRight(dir, string(filepath.Separator))
	parent := filepath.Dir(dir)
	dir = filepath.Base(dir)
	if parent != "." {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		if err := os.Chdir(parent); err != nil {
			return err
		}
		defer os.Chdir(cwd)
	}

	var ignoreMap map[string]struct{}
	if len(ignores) != 0 {
		ignoreMap = make(map[string]struct{}, len(ignores))
		for _, ign := range ignores {
			ignoreMap[ign] = struct{}{}
		}
	}

	dirs := []string{dir}
	for len(dirs) != 0 {
		// Pop dir from directories stack
		dir := dirs[len(dirs)-1]
		dirs = dirs[:len(dirs)-1]
		if dir == "" {
			continue
		}

		// Add dir header to tar.
		fi, err := os.Stat(dir)
		if err != nil {
			return err
		}
		hdr, err := tar.FileInfoHeader(fi, fi.Name())
		if err != nil {
			return err
		}
		slashDir := filepath.ToSlash(dir)
		hdr.Name = slashDir + "/"
		if err = tw.WriteHeader(hdr); err != nil {
			return err
		}

		// Add all the files in the directory to the archive.
		dirEnts, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		for _, de := range dirEnts {
			fname := de.Name()
			if _, found := ignoreMap[fname]; found {
				continue
			}

			pathName := filepath.Join(dir, fname)

			// If subdir, push onto stack to handle next iteration.
			if de.IsDir() {
				dirs = append(dirs, pathName)
				continue
			}

			// Skip non-regular files.
			if !de.Type().IsRegular() {
				continue
			}

			fi, err := de.Info()
			if err != nil {
				return err
			}

			// Create a new file header and write it to tar writer.
			if hdr, err = tar.FileInfoHeader(fi, fname); err != nil {
				return err
			}
			hdr.Name = path.Join(slashDir, fname)
			if err = tw.WriteHeader(hdr); err != nil {
				return err
			}

			// Copy file data into tar writer.
			f, err := os.Open(pathName)
			if err != nil {
				return err
			}
			if _, err = io.Copy(tw, f); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
	}
	return tw.Flush()
}

// Extract reads gzipped tar data from file into a directory.
func Extract(tarPath, targetDir string) error {
	f, err := os.Open(tarPath)
	if err != nil {
		return err
	}
	defer f.Close()
	return ExtractReader(f, targetDir)
}

// ExtractReader reads gzipped tar data from io.Reader and extracts it into the
// target directory.
func ExtractReader(r io.Reader, targetDir string) error {
	// gzip reader reads from archive file.
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gzr.Close()

	if targetDir == "" {
		targetDir = "."
	}

	uid := -1
	gid := -1
	isRoot := os.Getuid() == 0

	// tar reader reads from gzip.
	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		if header == nil {
			continue
		}

		if isRoot {
			uid = -1
			if header.Uname != "" {
				usr, err := user.Lookup(header.Uname)
				// Ignore error; user not on this host.
				if err == nil {
					uid, err = strconv.Atoi(usr.Uid)
					if err != nil {
						return err
					}
				}
			}
			gid = -1
			if header.Gname != "" {
				grp, err := user.LookupGroup(header.Gname)
				// Ignore error; group not on this host.
				if err == nil {
					gid, err = strconv.Atoi(grp.Gid)
					if err != nil {
						return err
					}
				}
			}
		}

		target := filepath.Join(targetDir, header.Name)
		fi := header.FileInfo()
		mode := fi.Mode()

		if mode.IsDir() {
			if _, err = os.Stat(target); err != nil {
				if err = os.MkdirAll(target, mode.Perm()); err != nil {
					return err
				}
				if uid != -1 || gid != -1 {
					// Ignore error; may not be allowed on NAS.
					_ = os.Chown(target, uid, gid)
				}
			}
		} else if mode.IsRegular() {
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, mode.Perm())
			if err != nil {
				return err
			}

			if _, err = io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()

			if uid != -1 || gid != -1 {
				// Ignore error; may not be allowed on NAS.
				_ = os.Chown(target, uid, gid)
			}
		}
	}

	return nil
}
