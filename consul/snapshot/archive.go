// The archive utilities manage the internal format of a snapshot, which is a
// zip-compressed file with the following contents:
//
// metadata.json - JSON-encoded snapshot metadata from Raft
// snapshot.data - Encoded snapshot data from Raft
// SHA256SUMS    - SHA-256 sums of the above two files
//
// The integrity information is automatically created and checked, and a failure
// there just looks like an error to the caller.
package snapshot

import (
	"archive/zip"
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"hash"
	"io"

	"github.com/hashicorp/raft"
)

// hashList manages a list of filenames and their hashes.
type hashList struct {
	hashes map[string]hash.Hash
}

// newHashList returns a new hashList.
func newHashList() *hashList {
	return &hashList{
		hashes: make(map[string]hash.Hash),
	}
}

// Add creates a new hash for the given file.
func (hl *hashList) Add(file string) hash.Hash {
	if existing, ok := hl.hashes[file]; ok {
		return existing
	}

	h := sha256.New()
	hl.hashes[file] = h
	return h
}

// Encode takes the current sum of all the hashes and saves the hash list as a
// SHA256SUMS-style text file.
func (hl *hashList) Encode(w io.Writer) error {
	for file, h := range hl.hashes {
		if _, err := fmt.Fprintf(w, "%x  %s\n", h.Sum([]byte{}), file); err != nil {
			return err
		}
	}
	return nil
}

// Decode reads a SHA256SUMS-style text file and checks the results against the
// current sums for all the hashes.
func (hl *hashList) Decode(r io.Reader) error {
	// Read the file and make sure everything in there has a matching hash.
	seen := make(map[string]struct{})
	s := bufio.NewScanner(r)
	for s.Scan() {
		sha := make([]byte, sha256.Size)
		var file string
		if _, err := fmt.Sscanf(s.Text(), "%x  %s", &sha, &file); err != nil {
			return err
		}

		h, ok := hl.hashes[file]
		if !ok {
			return fmt.Errorf("missing hash for %q", file)
		}
		if !bytes.Equal(sha, h.Sum([]byte{})) {
			return fmt.Errorf("hash check failed for %q", file)
		}
		seen[file] = struct{}{}
	}
	if err := s.Err(); err != nil {
		return err
	}

	// Make sure everything we had a hash for was seen.
	for file, _ := range hl.hashes {
		if _, ok := seen[file]; !ok {
			return fmt.Errorf("no hash found for %q", file)
		}
	}

	return nil
}

// write takes a zip writer and creates an archive with the snapshot metadata,
// the snapshot itself, and adds some integrity checking information.
func write(zipper *zip.Writer, metadata *raft.SnapshotMeta, snap io.Reader) error {
	// Create a hash list that we will use to write a SHA256SUMS file into
	// the archive.
	hl := newHashList()

	// Encode the snapshot metadata, which we need to feed back during a
	// restore.
	metaWriter, err := zipper.Create("metadata.json")
	if err != nil {
		return fmt.Errorf("failed to create snapshot metadata entry: %v", err)
	}
	metaHash := hl.Add("metadata.json")
	enc := json.NewEncoder(io.MultiWriter(metaHash, metaWriter))
	if err := enc.Encode(metadata); err != nil {
		return fmt.Errorf("failed to write snapshot metadata: %v", err)
	}

	// Streaming copy the serialized portion of the Raft snapshot.
	snapWriter, err := zipper.Create("snapshot.data")
	if err != nil {
		return fmt.Errorf("failed to create snapshot data entry: %v", err)
	}
	snapHash := hl.Add("snapshot.data")
	if _, err := io.Copy(io.MultiWriter(snapHash, snapWriter), snap); err != nil {
		return fmt.Errorf("failed to write snapshot data: %v", err)
	}

	// Create a SHA256SUMS file that we can use to verify on restore.
	shaWriter, err := zipper.Create("SHA256SUMS")
	if err != nil {
		return fmt.Errorf("failed to create snapshot hashes: %v", err)
	}
	if err := hl.Encode(shaWriter); err != nil {
		return fmt.Errorf("failed to write snapshot hashes: %v", err)
	}

	// Finalize the archive.
	if err := zipper.Close(); err != nil {
		return fmt.Errorf("failed to finalize snapshot: %v", err)
	}

	return nil
}

// openFile scans the archive for the given file and returns it, reporting an
// error if it's not in there.
func openFile(unzipper *zip.ReadCloser, file string) (*zip.File, error) {
	for _, f := range unzipper.File {
		if f.Name == file {
			return f, nil
		}
	}

	return nil, fmt.Errorf("failed to find %q", file)
}

// read takes a zip reader and extracts the snapshot metadata and the snapshot
// itself, and also checks the integrity of the data.
func read(unzipper *zip.ReadCloser) (*raft.SnapshotMeta, io.ReadCloser, error) {
	// Create a hash list that we will use to compare with the SHA256SUMS
	// file in the archive.
	hl := newHashList()

	// Open the metadata file.
	metaFile, err := openFile(unzipper, "metadata.json")
	if err != nil {
		return nil, nil, err
	}
	metaReader, err := metaFile.Open()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open snapshot metadata entry: %v", err)
	}
	defer metaReader.Close()

	// Decode the metadata and tee it through the hash so we can check it.
	var metadata raft.SnapshotMeta
	metaHash := hl.Add("metadata.json")
	dec := json.NewDecoder(io.TeeReader(metaReader, metaHash))
	if err := dec.Decode(&metadata); err != nil {
		return nil, nil, fmt.Errorf("failed to read snapshot metadata: %v", err)
	}

	// Get the snapshot file.
	snapFile, err := openFile(unzipper, "snapshot.data")
	if err != nil {
		return nil, nil, err
	}

	// Run through it once to get the hash. This kind of sucks, but we don't
	// want the ingestion of it into Raft to be where we discover things are
	// corrupt.
	snapHash := hl.Add("snapshot.data")
	snapReader, err := snapFile.Open()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open snapshot data entry: %v", err)
	}
	defer snapReader.Close()
	if _, err := io.Copy(snapHash, snapReader); err != nil {
		return nil, nil, fmt.Errorf("failed to hash snapshot data: %v", err)
	}

	// Verify all the hashes.
	shaFile, err := openFile(unzipper, "SHA256SUMS")
	if err != nil {
		return nil, nil, err
	}
	shaReader, err := shaFile.Open()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open snapshot hashes: %v", err)
	}
	defer shaReader.Close()
	if err := hl.Decode(shaReader); err != nil {
		return nil, nil, fmt.Errorf("failed checking integrity of snapshot: %v", err)
	}

	// Re-open the snapshot to pass up to the caller.
	snap, err := snapFile.Open()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open snapshot data entry: %v", err)
	}

	return &metadata, snap, nil
}
