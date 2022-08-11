package iso9660

/*
 These tests the exported functions
 We want to do full-in tests with files
*/

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/diskfs/go-diskfs/filesystem"
)

func getOpenMode(mode int) string {
	modes := make([]string, 0)
	if mode&os.O_CREATE == os.O_CREATE {
		modes = append(modes, "CREATE")
	}
	if mode&os.O_APPEND == os.O_APPEND {
		modes = append(modes, "APPEND")
	}
	if mode&os.O_RDWR == os.O_RDWR {
		modes = append(modes, "RDWR")
	} else {
		modes = append(modes, "RDONLY")
	}
	return strings.Join(modes, "|")
}

func getValidIso9660FSWorkspace() (*FileSystem, error) {
	// create the filesystem
	f, err := tmpIso9660File()
	if err != nil {
		return nil, fmt.Errorf("Failed to create iso9660 tmpfile: %v", err)
	}
	return Create(f, 0, 0, 2048, "")
}
func getValidIso9660FSUserWorkspace() (*FileSystem, error) {
	f, err := tmpIso9660File()
	if err != nil {
		return nil, fmt.Errorf("Failed to create iso9660 tmpfile: %v", err)
	}
	dir, err := ioutil.TempDir("", "myIso9660")
	if err != nil {
		return nil, fmt.Errorf("Failed to create iso9660 tmpfile: %v", err)
	}
	return Create(f, 0, 0, 2048, dir)
}
func getValidIso9660FSReadOnly() (*FileSystem, error) {
	f, err := os.Open(ISO9660File)
	if err != nil {
		return nil, fmt.Errorf("Failed to read iso9660 testfile %s: %v", ISO9660File, err)
	}
	return Read(f, 0, 0, 2048)
}
func getValidRockRidgeFSReadOnly() (*FileSystem, error) {
	f, err := os.Open(RockRidgeFile)
	if err != nil {
		return nil, fmt.Errorf("Failed to read iso9660 testfile %s: %v", RockRidgeFile, err)
	}
	return Read(f, 0, 0, 2048)
}

func tmpIso9660File() (*os.File, error) {
	filename := "iso9660_test.iso"
	f, err := ioutil.TempFile("", filename)
	if err != nil {
		return nil, fmt.Errorf("Failed to create tempfile %s :%v", filename, err)
	}
	return f, nil
}

func TestISO9660Type(t *testing.T) {
	fs := &FileSystem{}
	fstype := fs.Type()
	expected := filesystem.TypeISO9660
	if fstype != expected {
		t.Errorf("Type() returns %v instead of expected %v", fstype, expected)
	}
}

func TestIso9660Mkdir(t *testing.T) {
	t.Run("read-only", func(t *testing.T) {
		fs, err := getValidIso9660FSReadOnly()
		if err != nil {
			t.Fatalf("Failed to get read-only ISO9660 filesystem: %v", err)
		}
		err = fs.Mkdir("/abcdef")
		if err == nil {
			t.Errorf("received no error when trying to mkdir read-only filesystem")
		}
	})
	t.Run("workspace", func(t *testing.T) {
		fs, err := getValidIso9660FSWorkspace()
		if err != nil {
			t.Errorf("Failed to get workspace: %v", err)
		}
		existPath := "/abc"
		tests := []struct {
			fs   *FileSystem
			path string
			err  error
		}{
			{fs, "/abcdef", nil},                          // new one
			{fs, existPath, nil},                          // already exists
			{fs, path.Join(existPath, "bar/def/la"), nil}, // already exists
			{fs, "/a/b/c", nil},                           // already exists
		}

		// for fsw, we want to work at least once with a path that exists
		existPathFull := path.Join(fs.Workspace(), existPath)
		err = os.MkdirAll(existPathFull, 0o755)
		if err != nil {
			t.Fatalf("could not create path %s in workspace as %s: %v", existPath, existPathFull, err)
		}
		for _, tt := range tests {
			fs := tt.fs
			ws := fs.Workspace()
			err := fs.Mkdir(tt.path)
			if (err == nil && tt.err != nil) || (err != nil && err == nil) {
				t.Errorf("unexpected error mismatch. Actual: %v, expected: %v", err, tt.err)
			}
			// did the path exist?
			if ws != "" {
				fullPath := path.Join(ws, tt.path)
				if _, err := os.Stat(fullPath); os.IsNotExist(err) {
					t.Errorf("path did not exist after creation base %s, in workspace %s", tt.path, fullPath)
				}
			}
		}
	})
}

func TestIso9660Create(t *testing.T) {
	testFile, testError := ioutil.TempFile("", "iso9660_test")
	if testError != nil {
		t.Errorf("Failed to create workspace tmpfile: %v", testError)
	}
	defer os.RemoveAll(testFile.Name())
	testDir, testError := ioutil.TempDir("", "iso9660_test")
	if testError != nil {
		t.Errorf("Failed to create workspace tmpdir: %v", testError)
	}
	defer os.RemoveAll(testDir)

	missingDir, testError := ioutil.TempDir("", "iso9660_test")
	if testError != nil {
		t.Errorf("Failed to create workspace tmpdir: %v", testError)
	}
	os.RemoveAll(missingDir)

	tests := []struct {
		blocksize int64
		filesize  int64
		fs        *FileSystem
		err       error
		workdir   string
	}{
		{500, 6000, nil, fmt.Errorf("blocksize for ISO9660 must be"), ""},
		{513, 6000, nil, fmt.Errorf("blocksize for ISO9660 must be"), ""},
		{2048, 2048*MaxBlocks + 1, nil, fmt.Errorf("requested size is larger than maximum allowed ISO9660 size"), ""},
		{2048, 32*KB + 3*2048 - 1, nil, fmt.Errorf("requested size is smaller than minimum allowed ISO9660 size"), ""},
		{2048, 10000000, nil, fmt.Errorf("provided workspace is not a directory"), testFile.Name()},
		{2048, 10000000, nil, fmt.Errorf("could not stat working directory"), missingDir},
		{2048, 10000000, &FileSystem{}, nil, testDir},
		{2048, 10000000, &FileSystem{}, nil, ""},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(fmt.Sprintf("blocksize %d filesize %d", tt.blocksize, tt.filesize), func(t *testing.T) {
			// create the filesystem
			f, err := tmpIso9660File()
			if err != nil {
				t.Errorf("Failed to create iso9660 tmpfile: %v", err)
			}
			fs, err := Create(f, tt.filesize, 0, tt.blocksize, tt.workdir)
			defer os.Remove(f.Name())
			switch {
			case (err == nil && tt.err != nil) || (err != nil && tt.err == nil) || (err != nil && tt.err != nil && !strings.HasPrefix(err.Error(), tt.err.Error())):
				t.Errorf("Create(%s, %d, %d, %d): mismatched errors, actual %v expected %v", f.Name(), tt.filesize, 0, tt.blocksize, err, tt.err)
			case (fs == nil && tt.fs != nil) || (fs != nil && tt.fs == nil):
				t.Errorf("Create(%s, %d, %d, %d): mismatched fs, actual then expected", f.Name(), tt.filesize, 0, tt.blocksize)
				t.Logf("%v", fs)
				t.Logf("%v", tt.fs)
			}
			// we do not match the filesystems here, only check functional accuracy
		})
	}
}

func TestISO9660Read(t *testing.T) {
	// test cases:
	// - invalid blocksize
	// - invalid file size (too small and too big)
	// - valid file
	tests := []struct {
		blocksize  int64
		filesize   int64
		bytechange int64
		fs         *FileSystem
		err        error
	}{
		{500, 6000, -1, nil, fmt.Errorf("blocksize for ISO9660 must be")},
		{513, 6000, -1, nil, fmt.Errorf("blocksize for ISO9660 must be")},
		{512, MaxBlocks*2048 + 10000, -1, nil, fmt.Errorf("blocksize for ISO9660 must be")},
		{2048, 10000000, -1, &FileSystem{}, nil},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(fmt.Sprintf("blocksize %d filesize %d", tt.blocksize, tt.filesize), func(t *testing.T) {
			// get a temporary working file
			f, err := os.Open(ISO9660File)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			// create the filesystem
			fs, err := Read(f, tt.filesize, 0, tt.blocksize)
			switch {
			case (err == nil && tt.err != nil) || (err != nil && tt.err == nil) || (err != nil && tt.err != nil && !strings.HasPrefix(err.Error(), tt.err.Error())):
				t.Errorf("read(%s, %d, %d, %d): mismatched errors, actual %v expected %v", f.Name(), tt.filesize, 0, tt.blocksize, err, tt.err)
			case (fs == nil && tt.fs != nil) || (fs != nil && tt.fs == nil):
				t.Errorf("read(%s, %d, %d, %d): mismatched fs, actual then expected", f.Name(), tt.filesize, 0, tt.blocksize)
				t.Logf("%v", fs)
				t.Logf("%v", tt.fs)
			}
			// we do not match the filesystems here, only check functional accuracy
		})
	}
}

func TestIso9660ReadDir(t *testing.T) {
	type testList struct {
		fs    *FileSystem
		path  string
		count int
		first string
		last  string
		err   error
	}
	//nolint:thelper // this is not a helper function
	runTests := func(t *testing.T, tests []testList) {
		for _, tt := range tests {
			fi, err := tt.fs.ReadDir(tt.path)
			switch {
			case (err == nil && tt.err != nil) || (err != nil && tt.err == nil):
				t.Errorf("fs.ReadDir(%s): mismatched errors, actual %v expected %v", tt.path, err, tt.err)
			case len(fi) != tt.count:
				t.Errorf("fs.ReadDir(%s): mismatched directory received %d entries, expected %d", tt.path, len(fi), tt.count)
			case fi != nil && tt.count > 2 && fi[0].Name() != tt.first:
				t.Errorf("fs.ReadDir(%s): mismatched first non-self or parent entry, actual then expected", tt.path)
				t.Logf("%s", fi[0].Name())
				t.Logf("%s", tt.first)
			case fi != nil && tt.count > 0 && fi[len(fi)-1].Name() != tt.last:
				t.Errorf("fs.ReadDir(%s): mismatched last entry, actual then expected", tt.path)
				t.Logf("%s", fi[len(fi)-1].Name())
				t.Logf("%s", tt.last)
			}
		}
	}
	t.Run("read-only 9660", func(t *testing.T) {
		fs, err := getValidIso9660FSReadOnly()
		if err != nil {
			t.Errorf("Failed to get read-only ISO9660 filesystem: %v", err)
		}
		runTests(t, []testList{
			{fs, "/abcdef", 0, "", "", fmt.Errorf("directory does not exist")}, // does not exist
			// root should have 4 entries (since we do not pass back . and ..):
			// .
			// ..
			// /ABC
			// /BAR
			// /FOO
			// /README.MD;1
			{fs, "/", 5, "ABC", "README.MD", nil},                                   // exists
			{fs, "/ABC", 1, "", "LARGEFIL", nil},                                    // exists
			{fs, "/abc", 0, "", "LARGEFIL", fmt.Errorf("directory does not exist")}, // should not find rock ridge name
		},
		)
	})
	t.Run("read-only rock ridge", func(t *testing.T) {
		fs, err := getValidRockRidgeFSReadOnly()
		if err != nil {
			t.Errorf("Failed to get read-only Rock Ridge filesystem: %v", err)
		}
		runTests(t, []testList{
			{fs, "/abcdef", 0, "", "", fmt.Errorf("directory does not exist")}, // does not exist
			// root should have 4 entries (since we do not pass back . and ..):
			{fs, "/", 6, "abc", "README.md", nil},                                   // exists
			{fs, "/ABC", 0, "", "LARGEFIL", fmt.Errorf("directory does not exist")}, // should not find 8.3 name
			{fs, "/abc", 1, "", "largefile", nil},                                   // should find rock ridge name
			{fs, "/deep/a/b/c/d/e/f/g/h/i/j/k", 1, "file", "file", nil},             // should find a deep directory
			{fs, "/G", 0, "", "H", fmt.Errorf("directory does not exist")},          // relocated directory
			{fs, "/g", 0, "", "h", fmt.Errorf("directory does not exist")},          // relocated directory
		},
		)
	})
	t.Run("workspace", func(t *testing.T) {
		fs, err := getValidIso9660FSWorkspace()
		if err != nil {
			t.Errorf("Failed to get workspace: %v", err)
		}
		// make sure existPath exists in the workspace
		ws := fs.Workspace()
		existPath := "/abc"
		existPathWs := path.Join(ws, existPath)
		_ = os.MkdirAll(existPathWs, 0o755)
		// create files
		for i := 0; i < 10; i++ {
			filename := path.Join(existPathWs, fmt.Sprintf("filename_%d", i))
			contents := fmt.Sprintf("abcdefg %d", i)
			_ = os.WriteFile(filename, []byte(contents), 0o600)
		}
		runTests(t, []testList{
			{fs, "/abcdef", 0, "", "", fmt.Errorf("directory does not exist")}, // does not exist
			{fs, existPath, 10, "filename_0", "filename_9", nil},               // exists
		},
		)
	})
}

//nolint:gocyclo // we really do not care about the cyclomatic complexity of a test function. Maybe someday we will improve it.
func TestIso9660OpenFile(t *testing.T) {
	// opening directories and files for reading
	type testStruct struct {
		path     string
		mode     int
		expected string
		err      error
	}

	t.Run("read", func(t *testing.T) {
		//nolint:thelper // this is not a helper function
		runTests := func(t *testing.T, fs *FileSystem, tests []testStruct) {
			for _, tt := range tests {
				header := fmt.Sprintf("OpenFile(%s, %s)", tt.path, getOpenMode(tt.mode))
				reader, err := fs.OpenFile(tt.path, tt.mode)
				switch {
				case (err == nil && tt.err != nil) || (err != nil && tt.err == nil) || (err != nil && tt.err != nil && !strings.HasPrefix(err.Error(), tt.err.Error())):
					t.Errorf("%s: mismatched errors, actual: %v , expected: %v", header, err, tt.err)
				case reader == nil && (tt.err == nil || tt.expected != ""):
					t.Errorf("%s: Unexpected nil output", header)
				case reader != nil:
					b, err := io.ReadAll(reader)
					if err != nil {
						t.Errorf("%s: io.ReadAll(reader) unexpected error: %v", header, err)
					}
					if string(b) != tt.expected {
						t.Errorf("%s: mismatched contents, actual then expected", header)
						t.Log(string(b))
						t.Log(tt.expected)
					}
				}
			}
		}
		t.Run("read-only 9660", func(t *testing.T) {
			fs, err := getValidIso9660FSReadOnly()
			if err != nil {
				t.Errorf("Failed to get read-only ISO9660 filesystem: %v", err)
			}
			tests := []testStruct{
				// error opening a directory
				{"/", os.O_RDONLY, "", fmt.Errorf("cannot open directory %s as file", "/")},
				// open non-existent file for read or read write
				{"/abcdefg", os.O_RDONLY, "", fmt.Errorf("target file %s does not exist", "/abcdefg")},
				// open file for read or read write and check contents
				{"/FOO/FILENA01", os.O_RDONLY, "filename_1\n", nil},
				{"/FOO/FILENA75", os.O_RDONLY, "filename_9\n", nil},
				// rock ridge versions should not exist
				{"/README.md", os.O_RDONLY, "", fmt.Errorf("target file %s does not exist", "/README.md")},
			}
			runTests(t, fs, tests)
		})
		t.Run("read-only rock ridge", func(t *testing.T) {
			fs, err := getValidRockRidgeFSReadOnly()
			if err != nil {
				t.Errorf("Failed to get read-only Rock Ridge filesystem: %v", err)
			}
			tests := []testStruct{
				// error opening a directory
				{"/", os.O_RDONLY, "", fmt.Errorf("cannot open directory %s as file", "/")},
				// open non-existent file for read or read write
				{"/abcdefg", os.O_RDONLY, "", fmt.Errorf("target file %s does not exist", "/abcdefg")},
				// open file for read or read write and check contents
				{"/foo/filename_1", os.O_RDONLY, "filename_1\n", nil},
				{"/foo/filename_75", os.O_RDONLY, "filename_75\n", nil},
				// only rock ridge versions should exist
				{"/README.MD", os.O_RDONLY, "", fmt.Errorf("target file %s does not exist", "/README.MD")},
				{"/README.md", os.O_RDONLY, "README\n", nil},
			}
			runTests(t, fs, tests)
		})
		t.Run("workspace", func(t *testing.T) {
			fs, err := getValidIso9660FSWorkspace()
			if err != nil {
				t.Errorf("Failed to get workspace: %v", err)
			}
			// make sure our test files exist and have necessary content
			ws := fs.Workspace()
			subdir := "/FOO"
			_ = os.MkdirAll(path.Join(ws, subdir), 0o755)
			for i := 0; i <= 75; i++ {
				filename := fmt.Sprintf("FILENA%02d", i)
				content := fmt.Sprintf("filename_%d\n", i)
				_ = os.WriteFile(path.Join(ws, subdir, filename), []byte(content), 0o600)
			}
			tests := []testStruct{
				// error opening a directory
				{"/", os.O_RDONLY, "", fmt.Errorf("cannot open directory %s as file", "/")},
				// open non-existent file for read or read write
				{"/abcdefg", os.O_RDONLY, "", fmt.Errorf("target file %s does not exist", "/abcdefg")},
				// open file for read or read write and check contents
				{"/FOO/FILENA01", os.O_RDONLY, "filename_1\n", nil},
				{"/FOO/FILENA75", os.O_RDONLY, "filename_75\n", nil},
			}
			runTests(t, fs, tests)
		})
	})

	// write / create-and-write files and check contents
	// *** Write - writes right after last write or read
	// *** Read - reads right after last write or read
	// ** WriteAt - writes at specific location in file
	// ** ReadAt - reads at specific location in file
	t.Run("Write", func(t *testing.T) {
		t.Run("read-only", func(t *testing.T) {
			flags := []int{
				os.O_CREATE, os.O_APPEND, os.O_WRONLY, os.O_RDWR,
			}
			fs, err := getValidIso9660FSReadOnly()
			if err != nil {
				t.Errorf("Failed to get read-only ISO9660 filesystem: %v", err)
			}
			for _, m := range flags {
				_, err := fs.OpenFile("/NEWFILE", os.O_CREATE)
				if err == nil {
					t.Errorf("Did not return error when opening a file with read flag %v in read-only filesystem", m)
				}
			}
		})
		t.Run("workspace", func(t *testing.T) {
			fsTemp, err := getValidIso9660FSWorkspace()
			if err != nil {
				t.Errorf("Failed to get workspace: %v", err)
			}
			fsUser, err := getValidIso9660FSUserWorkspace()
			if err != nil {
				t.Errorf("Failed to get workspace: %v", err)
			}

			baseContent := "INITIAL DATA GALORE\n"
			editFile := "/EXISTS.TXT"
			tests := []struct {
				path      string
				mode      int
				beginning bool
				contents  string
				expected  string
				err       error
			}{
				//  - open for create file that does not exist (write contents, check that written)
				{"/abcdefg", os.O_RDWR | os.O_CREATE, false, "This is a test", "This is a test", nil},
				//  - open for readwrite file that does exist (write contents, check that overwritten)
				{editFile, os.O_RDWR, true, "This is a very long replacement string", "This is a very long replacement string", nil},
				{editFile, os.O_RDWR, true, "Two", "TwoTIAL DATA GALORE\n", nil},
				{editFile, os.O_RDWR, false, "This is a very long replacement string", "INITIAL DATA GALORE\nThis is a very long replacement string", nil},
				{editFile, os.O_RDWR, false, "Two", "INITIAL DATA GALORE\nTwo", nil},
				//  - open for append file that does exist (write contents, check that appended)
				{editFile, os.O_APPEND, false, "More", "", fmt.Errorf("write ")},
				{editFile, os.O_APPEND | os.O_RDWR, false, "More", "INITIAL DATA GALORE\nMore", nil},
				{editFile, os.O_APPEND, true, "More", "", fmt.Errorf("write ")},
				{editFile, os.O_APPEND | os.O_RDWR, true, "More", "INITIAL DATA GALORE\nMore", nil},
			}
			for i, tt := range tests {
				for _, fs := range []*FileSystem{fsTemp, fsUser} {
					filepath := path.Join(fs.Workspace(), tt.path)
					// remove any old file if it exists - ignore errors
					_ = os.Remove(filepath)
					// if the file is supposed to exist, create it and add its contents
					if tt.mode&os.O_CREATE != os.O_CREATE {
						_ = os.WriteFile(filepath, []byte(baseContent), 0o600)
					}
					header := fmt.Sprintf("%d: OpenFile(%s, %s, %t)", i, tt.path, getOpenMode(tt.mode), tt.beginning)
					readWriter, err := fs.OpenFile(tt.path, tt.mode)
					switch {
					case err != nil:
						t.Errorf("%s: unexpected error: %v", header, err)
					case readWriter == nil:
						t.Errorf("%s: Unexpected nil output", header)
					default:
						// read to the end of the file
						var offset int64
						_, err := readWriter.Seek(0, io.SeekEnd)
						if err != nil {
							t.Errorf("%s: Seek end of file gave unexpected error: %v", header, err)
							continue
						}
						if tt.beginning {
							offset, err = readWriter.Seek(0, io.SeekStart)
							if err != nil {
								t.Errorf("%s: Seek(0,io.SeekStart) unexpected error: %v", header, err)
								continue
							}
							if offset != 0 {
								t.Errorf("%s: Seek(0,io.SeekStart) reset to %d instead of %d", header, offset, 0)
								continue
							}
						}
						bWrite := []byte(tt.contents)
						written, writeErr := readWriter.Write(bWrite)
						_, _ = readWriter.Seek(0, io.SeekStart)
						bRead, readErr := io.ReadAll(readWriter)

						switch {
						case readErr != nil:
							t.Errorf("%s: io.ReadAll() unexpected error: %v", header, readErr)
						case (writeErr == nil && tt.err != nil) || (writeErr != nil && tt.err == nil) || (writeErr != nil && tt.err != nil && !strings.HasPrefix(writeErr.Error(), tt.err.Error())):
							t.Errorf("%s: readWriter.Write(b) mismatched errors, actual: %v , expected: %v", header, writeErr, tt.err)
						case written != len(bWrite) && tt.err == nil:
							t.Errorf("%s: readWriter.Write(b) wrote %d bytes instead of expected %d", header, written, len(bWrite))
						case string(bRead) != tt.expected && tt.err == nil:
							t.Errorf("%s: mismatched contents, actual then expected", header)
							t.Log(string(bRead))
							t.Log(tt.expected)
						}
					}
				}
			}
		})
	})
}

func TestIso9660Finalize(t *testing.T) {

}
