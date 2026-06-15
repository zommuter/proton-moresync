// radicale.go — Phase 2: Radicale collection adapter.
//
// generateRadicaleCollections materialises a separate Radicale collection root
// that references the backup tree. It emits the per-collection .Radicale.props
// markers (UUID + component-type: VADDRESSBOOK vs VCALENDAR) into the collection
// root only — NEVER into the git-versioned backup tree.
//
// Collection layout produced under collRoot:
//
//	<collRoot>/contacts/           — CardDAV address-book collection (one per backup)
//	    .Radicale.props            — {"tag":"VADDRESSBOOK",...}
//	<collRoot>/calendar/<cal-id>/  — CalDAV calendar collection (one per Proton calendar)
//	    .Radicale.props            — {"tag":"VCALENDAR",...}
//
// The canonical backup files (contacts/*.vcf, calendar/<cal-id>/*.ics) are
// referenced via OS symlinks from the collection root so Radicale can serve them.
// The backup tree is NEVER modified.
//
// CLI entry point: `backup gen-radicale-collections --backup-dir DIR --collection-root DIR`
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

// runGenRadicaleCollections is the CLI entry point for the
// "gen-radicale-collections" sub-command (invoked from main via dispatch).
func runGenRadicaleCollections(args []string) {
	fs := flag.NewFlagSet("gen-radicale-collections", flag.ExitOnError)
	backupDir := fs.String("backup-dir", "", "path to the canonical backup tree (required)")
	collRoot := fs.String("collection-root", "", "path to write the Radicale collection root (required)")
	_ = fs.Parse(args)

	if *backupDir == "" || *collRoot == "" {
		fmt.Fprintln(os.Stderr, "usage: backup gen-radicale-collections --backup-dir DIR --collection-root DIR")
		os.Exit(1)
	}
	if err := generateRadicaleCollections(*backupDir, *collRoot); err != nil {
		fmt.Fprintf(os.Stderr, "FATAL gen-radicale-collections: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Radicale collection root written to %s\n", *collRoot)
}

// generateRadicaleCollections builds a Radicale collection root at collRoot
// that serves the backup tree at backupRoot. The backup tree is never modified.
func generateRadicaleCollections(backupRoot, collRoot string) error {
	// 1. Address-book collection: contacts/*.vcf → collRoot/contacts/
	if err := generateAddressBook(backupRoot, collRoot); err != nil {
		return fmt.Errorf("address-book collection: %w", err)
	}

	// 2. Calendar collections: calendar/<cal-id>/ → collRoot/calendar/<cal-id>/
	if err := generateCalendarCollections(backupRoot, collRoot); err != nil {
		return fmt.Errorf("calendar collections: %w", err)
	}

	return nil
}

// generateAddressBook creates the contacts collection under collRoot/contacts/.
func generateAddressBook(backupRoot, collRoot string) error {
	srcDir := filepath.Join(backupRoot, "contacts")
	dstDir := filepath.Join(collRoot, "contacts")

	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return err
	}

	// Emit .Radicale.props for the collection.
	if err := writeRadicaleProps(dstDir, "VADDRESSBOOK", "Proton Contacts"); err != nil {
		return err
	}

	// Symlink each .vcf file into the collection directory.
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		if os.IsNotExist(err) {
			// No contacts dir yet — still valid, collection is empty.
			return nil
		}
		return err
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".vcf" {
			continue
		}
		src := filepath.Join(srcDir, e.Name())
		dst := filepath.Join(dstDir, e.Name())
		// Remove stale symlink if present (idempotent re-run).
		_ = os.Remove(dst)
		if err := os.Symlink(src, dst); err != nil {
			return fmt.Errorf("symlink %s → %s: %w", dst, src, err)
		}
	}
	return nil
}

// generateCalendarCollections creates one calendar collection per subdirectory
// of <backupRoot>/calendar/.
func generateCalendarCollections(backupRoot, collRoot string) error {
	calSrcRoot := filepath.Join(backupRoot, "calendar")
	calDstRoot := filepath.Join(collRoot, "calendar")

	calDirs, err := os.ReadDir(calSrcRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, d := range calDirs {
		if !d.IsDir() {
			continue
		}
		calID := d.Name()
		srcDir := filepath.Join(calSrcRoot, calID)
		dstDir := filepath.Join(calDstRoot, calID)

		if err := os.MkdirAll(dstDir, 0755); err != nil {
			return err
		}
		if err := writeRadicaleProps(dstDir, "VCALENDAR", calID); err != nil {
			return err
		}

		// Symlink each .ics file into the collection directory.
		entries, err := os.ReadDir(srcDir)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if e.IsDir() || filepath.Ext(e.Name()) != ".ics" {
				continue
			}
			src := filepath.Join(srcDir, e.Name())
			dst := filepath.Join(dstDir, e.Name())
			_ = os.Remove(dst)
			if err := os.Symlink(src, dst); err != nil {
				return fmt.Errorf("symlink %s → %s: %w", dst, src, err)
			}
		}
	}
	return nil
}

// propsJSON is the JSON shape for a .Radicale.props file.
// Radicale's multifilesystem storage parses this to identify collection types.
type propsJSON struct {
	Tag                            string `json:"tag"`
	DisplayName                    string `json:"D:displayname"`
	SupportedAddressData           string `json:"C:supported-address-data,omitempty"`
	SupportedCalendarComponentSets string `json:"C:supported-calendar-component-sets,omitempty"`
}

// writeRadicaleProps writes a .Radicale.props JSON file into dir.
func writeRadicaleProps(dir, tag, displayName string) error {
	props := propsJSON{
		Tag:         tag,
		DisplayName: displayName,
	}
	switch tag {
	case "VADDRESSBOOK":
		props.SupportedAddressData = "text/vcard"
	case "VCALENDAR":
		props.SupportedCalendarComponentSets = "VEVENT VTODO VJOURNAL"
	}

	data, err := json.MarshalIndent(props, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, ".Radicale.props"), data, 0644)
}
