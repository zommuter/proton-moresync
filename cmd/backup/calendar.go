package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	proton "github.com/ProtonMail/go-proton-api"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
)

type eventMeta struct {
	ProtonID          string                     `json:"proton_id"`
	CalendarID        string                     `json:"calendar_id"`
	SharedKeyPacket   string                     `json:"shared_key_packet"`
	CalendarKeyPacket string                     `json:"calendar_key_packet"`
	SharedEvents      []proton.CalendarEventPart `json:"shared_events"`
	CalendarEvents    []proton.CalendarEventPart `json:"calendar_events"`
	Version           int                        `json:"version"`
}

func backupCalendars(ctx context.Context, c *proton.Client, addrKRs map[string]*crypto.KeyRing, addresses []proton.Address, outDir string) error {
	calendars, err := c.GetCalendars(ctx)
	if err != nil {
		return fmt.Errorf("list calendars: %w", err)
	}

	addrByEmail := make(map[string]string)
	for _, addr := range addresses {
		addrByEmail[addr.Email] = addr.ID
	}

	for _, cal := range calendars {
		fmt.Printf("calendar %q (%s)\n", cal.Name, cal.ID)
		if err := backupCalendar(ctx, c, cal, addrKRs, addrByEmail, outDir); err != nil {
			fmt.Printf("  WARN: %v\n", err)
		}
	}
	return nil
}

func backupCalendar(ctx context.Context, c *proton.Client, cal proton.Calendar, addrKRs map[string]*crypto.KeyRing, addrByEmail map[string]string, outDir string) error {
	members, err := c.GetCalendarMembers(ctx, cal.ID)
	if err != nil {
		return fmt.Errorf("get members: %w", err)
	}

	var memberID string
	var calAddrKR *crypto.KeyRing
	for _, member := range members {
		addrID, ok := addrByEmail[member.Email]
		if !ok {
			continue
		}
		kr, ok := addrKRs[addrID]
		if !ok {
			continue
		}
		memberID = member.ID
		calAddrKR = kr
		break
	}
	if memberID == "" {
		return fmt.Errorf("no calendar member matches any local address")
	}

	calPassphrase, err := c.GetCalendarPassphrase(ctx, cal.ID)
	if err != nil {
		return fmt.Errorf("get passphrase: %w", err)
	}
	calPass, err := calPassphrase.Decrypt(memberID, calAddrKR)
	if err != nil {
		return fmt.Errorf("decrypt passphrase: %w", err)
	}

	calKeys, err := c.GetCalendarKeys(ctx, cal.ID)
	if err != nil {
		return fmt.Errorf("get keys: %w", err)
	}
	calKR, err := calKeys.Unlock(calPass)
	if err != nil {
		return fmt.Errorf("unlock calendar keys: %w", err)
	}

	total, skipped := 0, 0
	offset := 0
	for {
		events, err := c.GetCalendarEvents(ctx, cal.ID, offset, pageSize, url.Values{})
		if err != nil {
			return fmt.Errorf("list events at offset %d: %w", offset, err)
		}
		for _, event := range events {
			if err := writeEvent(event, cal.ID, calKR, calAddrKR, outDir); err != nil {
				fmt.Printf("  WARN: event %s: %v\n", event.ID, err)
				skipped++
				continue
			}
			total++
		}
		if len(events) < pageSize {
			break
		}
		offset += pageSize
	}
	fmt.Printf("  %d events written, %d skipped\n", total, skipped)
	return nil
}

func writeEvent(event proton.CalendarEvent, calID string, calKR, addrKR *crypto.KeyRing, outDir string) error {
	sharedKP := proton.DecodeKeyPacket(event.SharedKeyPacket)
	calKP := proton.DecodeKeyPacket(event.CalendarKeyPacket)

	// Decrypt all parts; SharedEvents[0] is the canonical event body.
	var sharedBody string
	for i, part := range event.SharedEvents {
		data, err := decryptPart(part, calKR, addrKR, sharedKP)
		if err != nil {
			continue
		}
		if i == 0 {
			sharedBody = data
		}
	}
	if sharedBody == "" {
		return fmt.Errorf("no SharedEvents decrypted")
	}

	// CalendarEvents hold personal annotations; decrypt best-effort.
	for _, part := range event.CalendarEvents {
		_, _ = decryptPart(part, calKR, addrKR, calKP)
	}

	uid := icsUID(sharedBody, event.ID)
	name := sanitize(uid)

	ics := wrapVCalendar(sharedBody)
	if err := writeFile(filepath.Join(outDir, "calendar", calID, name+".ics"), []byte(ics)); err != nil {
		return fmt.Errorf("write .ics: %w", err)
	}

	meta := eventMeta{
		ProtonID:          event.ID,
		CalendarID:        calID,
		SharedKeyPacket:   event.SharedKeyPacket,
		CalendarKeyPacket: event.CalendarKeyPacket,
		SharedEvents:      event.SharedEvents,
		CalendarEvents:    event.CalendarEvents,
		Version:           1,
	}
	metaData, _ := json.MarshalIndent(meta, "", "  ")
	if err := writeFile(filepath.Join(outDir, ".meta", "calendar", calID, name+".json"), append(metaData, '\n')); err != nil {
		return fmt.Errorf("write meta: %w", err)
	}
	return nil
}

// decryptPart replicates CalendarEventPart.Decode but returns the decrypted
// string. The upstream method uses a value receiver so its assignment is lost.
func decryptPart(part proton.CalendarEventPart, calKR, addrKR *crypto.KeyRing, kp []byte) (string, error) {
	data := part.Data

	if part.Type&proton.CalendarEventTypeEncrypted != 0 {
		var enc *crypto.PGPMessage
		if len(kp) > 0 {
			raw, err := base64.StdEncoding.DecodeString(data)
			if err != nil {
				return "", fmt.Errorf("base64 decode: %w", err)
			}
			enc = crypto.NewPGPSplitMessage(kp, raw).GetPGPMessage()
		} else {
			var err error
			if enc, err = crypto.NewPGPMessageFromArmored(data); err != nil {
				return "", fmt.Errorf("parse armored: %w", err)
			}
		}
		dec, err := calKR.Decrypt(enc, nil, crypto.GetUnixTime())
		if err != nil {
			return "", fmt.Errorf("decrypt: %w", err)
		}
		data = dec.GetString()
	}

	if part.Type&proton.CalendarEventTypeSigned != 0 {
		sig, err := crypto.NewPGPSignatureFromArmored(part.Signature)
		if err != nil {
			return "", fmt.Errorf("parse sig: %w", err)
		}
		if err := addrKR.VerifyDetached(crypto.NewPlainMessageFromString(data), sig, crypto.GetUnixTime()); err != nil {
			return "", fmt.Errorf("verify sig: %w", err)
		}
	}

	return data, nil
}

func icsUID(content, fallback string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.HasPrefix(line, "UID:") {
			return strings.TrimPrefix(line, "UID:")
		}
	}
	return fallback
}

func wrapVCalendar(content string) string {
	if strings.Contains(content, "BEGIN:VCALENDAR") {
		return content
	}
	if !strings.Contains(content, "BEGIN:VEVENT") {
		content = "BEGIN:VEVENT\r\n" + content + "END:VEVENT\r\n"
	}
	return "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//proton-moresync//backup//EN\r\n" +
		content + "END:VCALENDAR\r\n"
}
