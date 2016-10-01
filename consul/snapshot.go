package consul

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/hashicorp/raft"
)

// Snapshot is a structure that holds state about a temporary file that is used
// to hold a snapshot. By using an intermediate file we avoid holding everything
// in memory.
type Snapshot struct {
	file *os.File
}

// NewSnapshot takes a state snapshot of the given Raft instance into a
// temporary file and returns an object that gives access to the file as
// an io.Reader. You must arrange to call Close() on the returned object or
// else you will leak a temporary file.
func NewSnapshot(logger *log.Logger, r *raft.Raft) (*Snapshot, error) {
	// Take the snapshot.
	future := r.Snapshot()
	if err := future.Error(); err != nil {
		return nil, fmt.Errorf("Raft error when taking snapshot: %v", err)
	}

	// Open up the snapshot.
	metadata, snapReader, err := future.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open snapshot: %v:", err)
	}
	defer func() {
		if err := snapReader.Close(); err != nil {
			logger.Printf("[ERR] consul: Failed to close Raft snapshot: %v", err)
		}
	}()

	// Make a scratch file to receive the contents so that we don't buffer
	// everything in memory. This gets deleted in Close() since we keep it
	// around for re-reading.
	archive, err := ioutil.TempFile("", "consul-snapshot")
	if err != nil {
		return nil, fmt.Errorf("failed to create snapshot file: %v", err)
	}

	// If anything goes wrong after this point, we will attempt to clean up
	// the temp file. The happy path will disarm this.
	var keep bool
	defer func() {
		if keep {
			return
		}

		if err := os.Remove(archive.Name()); err != nil {
			logger.Printf("[ERR] consul: Failed to clean up temp snapshot: %v", err)
		}
	}()

	// Start an archive. This is a handy way to compress the potentially
	// large state dump and save out some extra metadata in a format that
	// humans could inspect if needed.
	zipper := zip.NewWriter(archive)

	// TODO - create a SHA256SUMS file

	// Encode the snapshot metadata, which we need to feed back during a
	// restore.
	metaWriter, err := zipper.Create("metadata.json")
	if err != nil {
		return nil, fmt.Errorf("failed to create snapshot metadata entry: %v", err)
	}
	enc := json.NewEncoder(metaWriter)
	if err := enc.Encode(&metadata); err != nil {
		return nil, fmt.Errorf("failed to write snapshot metadata: %v", err)
	}

	// Streaming copy the serialized portion of the Raft snapshot.
	snapWriter, err := zipper.Create("snapshot.data")
	if err != nil {
		return nil, fmt.Errorf("failed to create data entry: %v", err)
	}
	if _, err := io.Copy(snapWriter, snapReader); err != nil {
		return nil, fmt.Errorf("failed to write data: %v", err)
	}

	// Finalize the archive.
	if err := zipper.Close(); err != nil {
		return nil, fmt.Errorf("failed to finalize snapshot: %v", err)
	}

	// Rewind the file so it's ready to be read again.
	if _, err := archive.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("failed to rewind snapshot: %v", err)
	}

	keep = true
	return &Snapshot{archive}, nil
}

// XXX - TODO
func (s *Snapshot) Read(p []byte) (n int, err error) {
	if s == nil {
		return 0, io.EOF
	}

	return s.file.Read(p)
}

// Close closes the snapshot and removes any temporary storage associated with
// it. You must arrange to call this whenever NewSnapshot() has been called
// successfully. This is safe to call on a nil snapshot.
func (s *Snapshot) Close() error {
	if s == nil {
		return nil
	}

	if err := s.file.Close(); err != nil {
		return err
	}
	return os.Remove(s.file.Name())
}

// RestoreSnapshot takes the snapshot from the reader and attempts to apply it
// to the given Raft instance.
func RestoreSnapshot(logger *log.Logger, reader io.Reader, r *raft.Raft) error {
	// Make a scratch file to receive the contents so that we don't buffer
	// everything in memory. Also, the zip reader needs the size before it
	// can process it.
	archive, err := ioutil.TempFile("", "consul-snapshot")
	if err != nil {
		return fmt.Errorf("failed to create snapshot file: %v", err)
	}
	defer func() {
		if err := os.Remove(archive.Name()); err != nil {
			logger.Printf("[ERR] consul: Failed to clean up temp snapshot: %v", err)
		}
	}()
	if _, err := io.Copy(archive, reader); err != nil {
		return fmt.Errorf("failed to write snapshot: %v", err)
	}
	if err := archive.Close(); err != nil {
		return fmt.Errorf("failed to close snapshot: %v", err)
	}

	// Now we can open the snapshot from the temp file.
	unzipper, err := zip.OpenReader(archive.Name())
	if err != nil {
		return fmt.Errorf("failed to open snapshot file: %v", err)
	}
	defer func() {
		if err := unzipper.Close(); err != nil {
			logger.Printf("[ERR] consul: Failed to close snapshot file: %v", err)
		}
	}()

	// Look for the metadata and decode it.
	var metadata raft.SnapshotMeta
	for _, f := range unzipper.File {
		if f.Name == "metadata.json" {
			metaReader, err := f.Open()
			if err != nil {
				return fmt.Errorf("failed to open snapshot metadata: %v", err)
			}
			defer metaReader.Close()

			dec := json.NewDecoder(metaReader)
			if err := dec.Decode(&metadata); err != nil {
				return fmt.Errorf("failed to read snapshot metadata: %v", err)
			}
			goto DATA
		}
	}
	return fmt.Errorf("failed to find snapshot metadata")

	// Look for the data and do the restore.
DATA:
	for _, f := range unzipper.File {
		if f.Name == "snapshot.data" {
			snapReader, err := f.Open()
			if err != nil {
				return fmt.Errorf("failed to open snapshot data: %v", err)
			}
			defer snapReader.Close()

			future := r.Restore(&metadata, snapReader, 5*time.Minute)
			if err := future.Error(); err != nil {
				return fmt.Errorf("Raft error when restoring snapshot: %v", err)
			}
			return nil
		}
	}
	return fmt.Errorf("failed to find snapshot data")
}
