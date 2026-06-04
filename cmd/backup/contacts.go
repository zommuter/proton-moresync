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
	ProtonID string `json:"proton_id"`
	Version  int    `json:"version"`
}

func backupContacts(ctx context.Context, c *proton.Client, addrKRs map[string]*crypto.KeyRing, outDir string) error {
	total, skipped := 0, 0
	offset := 0
	for {
		batch, err := c.GetContacts(ctx, offset, pageSize)
		if err != nil {
			return fmt.Errorf("list contacts at offset %d: %w", offset, err)
		}
		for _, stub := range batch {
			if err := writeContact(ctx, c, stub.ID, addrKRs, outDir); err != nil {
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

func writeContact(ctx context.Context, c *proton.Client, id string, addrKRs map[string]*crypto.KeyRing, outDir string) error {
	contact, err := c.GetContact(ctx, id)
	if err != nil {
		return fmt.Errorf("fetch: %w", err)
	}

	var vc vcard.Card
	var decErr error
	for _, kr := range addrKRs {
		vc, decErr = contact.Cards.Merge(kr)
		if decErr == nil {
			break
		}
	}
	if decErr != nil {
		return fmt.Errorf("decrypt (tried %d keyrings): %w", len(addrKRs), decErr)
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

	meta := contactMeta{ProtonID: id, Version: 1}
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
