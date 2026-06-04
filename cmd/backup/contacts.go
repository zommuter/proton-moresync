package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	proton "github.com/ProtonMail/go-proton-api"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
	vcard "github.com/emersion/go-vcard"
)

type contactMeta struct {
	ProtonID string       `json:"proton_id"`
	Cards    proton.Cards `json:"cards"`
	Version  int          `json:"version"`
}

// backupContacts fetches and decrypts all contacts. contactKR must combine the
// user key (for encrypted cards) and all address keys (for signed cards).
func backupContacts(ctx context.Context, c *proton.Client, contactKR *crypto.KeyRing, outDir string) error {
	total, skipped := 0, 0
	offset := 0
	for {
		batch, err := c.GetContacts(ctx, offset, pageSize)
		if err != nil {
			return fmt.Errorf("list contacts at offset %d: %w", offset, err)
		}
		for _, stub := range batch {
			if err := writeContact(ctx, c, stub.ID, contactKR, outDir); err != nil {
				fmt.Printf("  WARN: contact %s: %v\n", stub.ID, err)
				skipped++
				continue
			}
			total++
		}
		if len(batch) < pageSize {
			break
		}
		offset += pageSize
	}
	fmt.Printf("contacts: %d written, %d skipped\n", total, skipped)
	return nil
}

func writeContact(ctx context.Context, c *proton.Client, id string, contactKR *crypto.KeyRing, outDir string) error {
	contact, err := c.GetContact(ctx, id)
	if err != nil {
		return fmt.Errorf("fetch: %w", err)
	}

	vc, err := contact.Cards.Merge(contactKR)
	if err != nil {
		return fmt.Errorf("decrypt: %w", err)
	}

	uid := vcardUID(vc, id)
	name := sanitize(uid)

	var buf bytes.Buffer
	if err := vcard.NewEncoder(&buf).Encode(vc); err != nil {
		return fmt.Errorf("encode vCard: %w", err)
	}

	if err := writeFile(filepath.Join(outDir, "contacts", name+".vcf"), buf.Bytes()); err != nil {
		return fmt.Errorf("write .vcf: %w", err)
	}

	meta := contactMeta{ProtonID: id, Cards: contact.Cards, Version: 1}
	metaData, _ := json.MarshalIndent(meta, "", "  ")
	if err := writeFile(filepath.Join(outDir, ".meta", "contacts", name+".json"), append(metaData, '\n')); err != nil {
		return fmt.Errorf("write meta: %w", err)
	}
	return nil
}

func vcardUID(card vcard.Card, fallback string) string {
	if f := card.Get(vcard.FieldUID); f != nil && f.Value != "" {
		return f.Value
	}
	return fallback
}
