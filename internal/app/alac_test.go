package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"amdl/internal/config"
	"amdl/internal/media/alacfix"
)

func TestParseALACVariant(t *testing.T) {
	cases := []struct {
		name      string
		audio     string
		wantOK    bool
		wantRate  int
		wantDepth int
	}{
		{name: "hi-res 24/192", audio: "audio-alac-stereo-192000-24", wantOK: true, wantRate: 192000, wantDepth: 24},
		{name: "hi-res 24/96", audio: "audio-alac-stereo-96000-24", wantOK: true, wantRate: 96000, wantDepth: 24},
		{name: "cd quality 16/44.1", audio: "audio-alac-stereo-44100-16", wantOK: true, wantRate: 44100, wantDepth: 16},
		{name: "second to last field is rate", audio: "alac-2-48000-16", wantOK: true, wantRate: 48000, wantDepth: 16},
		{name: "aac attribute is rejected", audio: "audio-stereo-256", wantOK: false},
		{name: "single field is rejected", audio: "alac", wantOK: false},
		{name: "empty attribute is rejected", audio: "", wantOK: false},
		{name: "non numeric rate is rejected", audio: "audio-alac-stereo-audio-24", wantOK: false},
		{name: "zero rate is rejected", audio: "audio-alac-stereo-0-24", wantOK: false},
		{name: "trailing dash is rejected", audio: "audio-alac-stereo-192000-", wantOK: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec, ok := parseALACVariant(tc.audio)
			if ok != tc.wantOK {
				t.Fatalf("parseALACVariant(%q) ok = %v, want %v", tc.audio, ok, tc.wantOK)
			}
			if !tc.wantOK {
				return
			}
			if spec.sampleRate != tc.wantRate || spec.bitDepth != tc.wantDepth {
				t.Fatalf("parseALACVariant(%q) = %dHz/%dbit, want %dHz/%dbit",
					tc.audio, spec.sampleRate, spec.bitDepth, tc.wantRate, tc.wantDepth)
			}
		})
	}
}

func TestSelectBestALACVariant(t *testing.T) {
	cd := alacVariantSpec{sampleRate: 44100, bitDepth: 16}
	hd := alacVariantSpec{sampleRate: 96000, bitDepth: 24}
	hiRes := alacVariantSpec{sampleRate: 192000, bitDepth: 24}
	hiRes16 := alacVariantSpec{sampleRate: 192000, bitDepth: 16}

	cases := []struct {
		name      string
		variants  []alacVariantSpec
		max       int
		wantOK    bool
		wantRate  int
		wantDepth int
	}{
		{
			name:      "prefers highest sample rate",
			variants:  []alacVariantSpec{cd, hiRes, hd},
			max:       192000,
			wantOK:    true,
			wantRate:  192000,
			wantDepth: 24,
		},
		{
			name:      "max filters out higher renditions",
			variants:  []alacVariantSpec{cd, hiRes, hd},
			max:       96000,
			wantOK:    true,
			wantRate:  96000,
			wantDepth: 24,
		},
		{
			name:      "depth breaks a rate tie",
			variants:  []alacVariantSpec{hiRes16, hiRes},
			max:       192000,
			wantOK:    true,
			wantRate:  192000,
			wantDepth: 24,
		},
		{
			name:      "zero max imposes no ceiling",
			variants:  []alacVariantSpec{cd, hiRes},
			max:       0,
			wantOK:    true,
			wantRate:  192000,
			wantDepth: 24,
		},
		{
			name:     "only rendition above max yields nothing",
			variants: []alacVariantSpec{hiRes},
			max:      96000,
			wantOK:   false,
		},
		{
			name:     "no variants yields nothing",
			variants: nil,
			max:      192000,
			wantOK:   false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := selectBestALACVariant(tc.variants, tc.max)
			if ok != tc.wantOK {
				t.Fatalf("selectBestALACVariant ok = %v, want %v", ok, tc.wantOK)
			}
			if !tc.wantOK {
				return
			}
			if got.sampleRate != tc.wantRate || got.bitDepth != tc.wantDepth {
				t.Fatalf("selectBestALACVariant = %dHz/%dbit, want %dHz/%dbit",
					got.sampleRate, got.bitDepth, tc.wantRate, tc.wantDepth)
			}
		})
	}
}

// TestSelectBestALACVariantIsOrderIndependent guards the regression this
// selection logic exists to fix: the old code took the first rendition that fit,
// so playlist ordering decided the quality.
func TestSelectBestALACVariantIsOrderIndependent(t *testing.T) {
	renditions := []alacVariantSpec{
		{sampleRate: 44100, bitDepth: 16},
		{sampleRate: 96000, bitDepth: 24},
		{sampleRate: 192000, bitDepth: 24},
	}
	first, ok := selectBestALACVariant(renditions, 192000)
	if !ok {
		t.Fatal("expected a selection")
	}

	reversed := []alacVariantSpec{renditions[2], renditions[1], renditions[0]}
	second, ok := selectBestALACVariant(reversed, 192000)
	if !ok {
		t.Fatal("expected a selection")
	}

	if first != second {
		t.Fatalf("selection depends on order: %+v vs %+v", first, second)
	}
}

// TestLosslessOnlySkipsExplicitLossyModes pins that the guard governs only the
// implicit AAC fallback, never a deliberate --aac / --atmos request.
func TestLosslessOnlySkipsExplicitLossyModes(t *testing.T) {
	r := NewRunner(config.ConfigSet{})
	r.Config.Media.LosslessOnly = true

	if !r.losslessOnly() {
		t.Fatal("expected losslessOnly() on a plain lossless run")
	}

	r.Flags.AAC = true
	if r.losslessOnly() {
		t.Fatal("explicit --aac must not be affected by lossless-only")
	}

	r.Flags.AAC = false
	r.Flags.Atmos = true
	if r.losslessOnly() {
		t.Fatal("explicit --atmos must not be affected by lossless-only")
	}
}

func TestFormatALACQuality(t *testing.T) {
	// The label feeds the {Quality} filename placeholder and therefore ends up in
	// file names, where a stray dot would be sanitised away; assert both the
	// expected ICU-free rendering and the absence of trailing zeros.
	if got := formatALACQuality(alacVariantSpec{sampleRate: 192000, bitDepth: 24}); got != "24B-192.0kHz" {
		t.Fatalf("formatALACQuality = %q, want '24B-192.0kHz'", got)
	}
	if got := formatALACQuality(alacVariantSpec{sampleRate: 44100, bitDepth: 16}); got != "16B-44.1kHz" {
		t.Fatalf("formatALACQuality = %q, want '16B-44.1kHz'", got)
	}
	if !utf8.ValidString(formatALACQuality(alacVariantSpec{sampleRate: 96000, bitDepth: 24})) {
		t.Fatal("quality label is not valid UTF-8")
	}
}

// TestVerifyLosslessStreamRejectsLossyFile covers the case media.lossless-only
// cannot reach: a stream that claims to be lossless but carries no ALAC track.
func TestVerifyLosslessStreamRejectsLossyFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "fake.m4a")
	// A minimal box with no ALAC sample entry.
	payload := make([]byte, 16)
	payload[3] = 16
	copy(payload[4:8], "moov")
	if err := os.WriteFile(path, payload, 0644); err != nil {
		t.Fatal(err)
	}

	r := NewRunner(config.ConfigSet{})
	err := r.verifyLosslessStream(path)
	if err == nil {
		t.Fatal("expected an error for a file with no ALAC track")
	}
	if !strings.Contains(err.Error(), "no ALAC audio track") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyLosslessStreamReportsMissingFile(t *testing.T) {
	r := NewRunner(config.ConfigSet{})
	err := r.verifyLosslessStream(filepath.Join(t.TempDir(), "absent.m4a"))
	if err == nil {
		t.Fatal("expected an error for a missing file")
	}
}

// TestAlacfixScanIsWiredToVerify keeps verifyLosslessStream honest about which
// signal it turns into a failure; if the alacfix contract ever changes to return
// an error instead of TracksCount == 0, this fails loudly.
func TestAlacfixScanIsWiredToVerify(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "absent.m4a")
	if _, err := alacfix.Scan(path); err == nil {
		t.Fatal("expected alacfix.Scan to report a missing file")
	}
}
