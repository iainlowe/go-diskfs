package iso9660

import (
	"io"
	"testing"
)

func TestFileRead(t *testing.T) {
	// pretty simple: never should be able to write as it is a read-only filesystem
	// we use
	f, content := GetTestFile(t)

	b := make([]byte, 20)
	read, err := f.Read(b)
	if read != 0 && err != io.EOF {
		t.Errorf("received unexpected error when reading: %v", err)
	}
	if read != len(content) {
		t.Errorf("read %d bytes instead of expected %d", read, len(content))
	}
	bString := string(b[:read])
	if bString != content {
		t.Errorf("Mismatched content:\nActual: '%s'\nExpected: '%s'", bString, content)
	}
}

func TestLargeFileCopy(t *testing.T) {
	f, size := GetLargeTestFile(t)

	copied, err := io.Copy(io.Discard, f)
	if err != nil {
		t.Errorf("received unexpected error when copying: %v", err)
	}
	if copied != int64(size) {
		t.Errorf("copied %d bytes instead of expected %d", copied, size)
	}
}

func TestFileWrite(t *testing.T) {
	// pretty simple: never should be able to write as it is a read-only filesystem
	f := &File{}
	b := make([]byte, 8)
	written, err := f.Write(b)
	if err == nil {
		t.Errorf("received no error when should have been prevented from writing")
	}
	if written != 0 {
		t.Errorf("wrote %d bytes instead of expected %d", written, 0)
	}
}
