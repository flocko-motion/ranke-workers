package contacts

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	rankedb "github.com/flocko-motion/rankedb/worker"
)

// Process unpacks contacts and photos from a Google Takeout archive.
// VCF files become L0 source/data nodes; images become L0 source/media
// nodes linked to their matching contact by filename stem.
func Process(ctx context.Context, client *rankedb.Client, bulkNodeID, configID, runID string, archive []byte) error {
	entries, err := unpackTarGz(archive)
	if err != nil {
		return fmt.Errorf("unpack archive: %w", err)
	}

	contactNodes := map[string]string{} // filename stem → node ID
	var photos []archiveEntry

	for _, e := range entries {
		ext := strings.ToLower(filepath.Ext(e.Name))
		if ext == ".vcf" {
			id, err := createContactNode(ctx, client, e, bulkNodeID, configID, runID)
			if err != nil {
				fmt.Printf("  SKIP %s: %v\n", e.Name, err)
				continue
			}
			stem := strings.TrimSuffix(filepath.Base(e.Name), ext)
			contactNodes[stem] = id
		} else if isImage(ext) {
			photos = append(photos, e)
		}
	}

	for _, p := range photos {
		stem := strings.TrimSuffix(filepath.Base(p.Name), filepath.Ext(p.Name))
		parentID, ok := contactNodes[stem]
		if !ok {
			fmt.Printf("  SKIP photo %s: no matching contact\n", p.Name)
			continue
		}
		if err := createPhotoNode(ctx, client, p, parentID, configID, runID); err != nil {
			fmt.Printf("  SKIP photo %s: %v\n", p.Name, err)
		}
	}

	fmt.Printf("Contacts: created %d from %s.\n", len(contactNodes), bulkNodeID)
	return nil
}

type archiveEntry struct {
	Name string
	Data []byte
}

func unpackTarGz(data []byte) ([]archiveEntry, error) {
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("gzip: %w", err)
	}
	defer gr.Close()

	var entries []archiveEntry
	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("tar: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		body, err := io.ReadAll(tr)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", hdr.Name, err)
		}
		entries = append(entries, archiveEntry{Name: hdr.Name, Data: body})
	}
	return entries, nil
}

func createContactNode(ctx context.Context, client *rankedb.Client, e archiveEntry, bulkNodeID, configID, runID string) (string, error) {
	name := filepath.Base(e.Name)

	resp, err := client.CreateNode(ctx, rankedb.CreateNodeRequest{
		Level:          0,
		ContentClass:   "source",
		ContentType:    "data",
		EncodingClass:  "text",
		EncodingFormat: "vcf",
		Content:        rankedb.Ptr(string(e.Data)),
		OriginalName:   rankedb.Ptr(name),
		Origin:         rankedb.Ptr("google-takeout/contacts"),
		Edges: []rankedb.EdgeSpec{
			{Type: "provenance/input", SourceNodeID: bulkNodeID, RunID: rankedb.Ptr(runID)},
			{Type: "provenance/worker", SourceNodeID: configID, RunID: rankedb.Ptr(runID)},
		},
	})
	if err != nil {
		return "", err
	}
	fmt.Printf("  + contact %s → %s\n", name, resp.Id)
	return resp.Id, nil
}

func createPhotoNode(ctx context.Context, client *rankedb.Client, e archiveEntry, contactNodeID, configID, runID string) error {
	name := filepath.Base(e.Name)
	ext := strings.ToLower(filepath.Ext(name))
	format := strings.TrimPrefix(ext, ".")

	resp, err := client.CreateNode(ctx, rankedb.CreateNodeRequest{
		Level:          0,
		ContentClass:   "source",
		ContentType:    "media",
		EncodingClass:  "image",
		EncodingFormat: format,
		Content:        rankedb.Ptr(string(e.Data)),
		OriginalName:   rankedb.Ptr(name),
		Origin:         rankedb.Ptr("google-takeout/contacts"),
		Edges: []rankedb.EdgeSpec{
			{Type: "provenance/input", SourceNodeID: contactNodeID, RunID: rankedb.Ptr(runID)},
			{Type: "provenance/worker", SourceNodeID: configID, RunID: rankedb.Ptr(runID)},
		},
	})
	if err != nil {
		return err
	}
	fmt.Printf("  + photo %s → %s (parent: %s)\n", name, resp.Id, contactNodeID)
	return nil
}

func isImage(ext string) bool {
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".tiff":
		return true
	}
	return false
}
