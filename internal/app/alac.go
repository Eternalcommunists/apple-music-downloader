package app

import (
	"fmt"
	"strconv"
	"strings"

	"amdl/internal/media/alacfix"
)

// alacVariantSpec describes one ALAC rendition advertised by a master playlist.
// Apple renders the audio attribute as `audio-alac-stereo-<sampleRate>-<bitDepth>`,
// e.g. `audio-alac-stereo-192000-24`. Both trailing fields must parse for a
// variant to be treated as real lossless; anything else is ignored rather than
// risk picking an AAC rendition.
type alacVariantSpec struct {
	sampleRate int // Hz, e.g. 44100 / 48000 / 96000 / 192000
	bitDepth   int // bits per sample, e.g. 16 / 24
	bandwidth  int // declared average bandwidth, used only as a tie-breaker
}

// betterALAC reports whether candidate is preferable to best. Preference order:
// sample rate, then bit depth, then bandwidth. Because it is a strict ordering,
// the first acceptable variant wins a tie and iteration order stays stable.
func betterALAC(candidate, best alacVariantSpec) bool {
	if candidate.sampleRate != best.sampleRate {
		return candidate.sampleRate > best.sampleRate
	}
	if candidate.bitDepth != best.bitDepth {
		return candidate.bitDepth > best.bitDepth
	}
	return candidate.bandwidth > best.bandwidth
}

// parseALACVariant extracts the sample-rate/bit-depth pair from an ALAC variant's
// Audio attribute. It reports false for any variant that is not parseable ALAC so
// the caller can skip it instead of silently downgrading to another codec.
func parseALACVariant(audio string) (alacVariantSpec, bool) {
	split := strings.Split(audio, "-")
	if len(split) < 2 {
		return alacVariantSpec{}, false
	}
	sampleRate, err := strconv.Atoi(split[len(split)-2])
	if err != nil || sampleRate <= 0 {
		return alacVariantSpec{}, false
	}
	bitDepth, err := strconv.Atoi(split[len(split)-1])
	if err != nil || bitDepth <= 0 {
		return alacVariantSpec{}, false
	}
	return alacVariantSpec{sampleRate: sampleRate, bitDepth: bitDepth}, true
}

// selectBestALACVariant returns the highest-quality ALAC variant whose sample
// rate does not exceed maxSampleRate (0 means no ceiling). It reports false when
// no ALAC variant fits, so the caller can treat the track as unavailable rather
// than fall back to a lossy stream.
func selectBestALACVariant(variants []alacVariantSpec, maxSampleRate int) (alacVariantSpec, bool) {
	var best alacVariantSpec
	found := false
	for _, v := range variants {
		if maxSampleRate > 0 && v.sampleRate > maxSampleRate {
			continue
		}
		if !found || betterALAC(v, best) {
			best = v
			found = true
		}
	}
	return best, found
}

// losslessOnly reports whether a track must never silently fall back to a lossy
// stream. It never applies to an explicit --aac or --atmos request, which selects
// a lossy codec on purpose.
func (r *Runner) losslessOnly() bool {
	return r.Config.Media.LosslessOnly && !r.Flags.AAC && !r.Flags.Atmos
}

// formatALACQuality renders the human-readable quality label used by the
// {Quality} placeholder, e.g. "24B-192.0kHz".
func formatALACQuality(v alacVariantSpec) string {
	return fmt.Sprintf("%dB-%.1fkHz", v.bitDepth, float64(v.sampleRate)/1000.0)
}

// verifyLosslessStream confirms the downloaded file really carries an ALAC track
// and reports malformed packets left in it. It runs on every lossless download
// when media.alac-verify is enabled, independently of media.alac-fix, so damage
// is surfaced even when automatic repair is switched off.
func (r *Runner) verifyLosslessStream(trackPath string) error {
	res, err := alacfix.Scan(trackPath)
	if err != nil {
		return fmt.Errorf("scan ALAC stream: %w", err)
	}
	if res.TracksCount == 0 {
		return fmt.Errorf("no ALAC audio track found; file may have silently fallen back to a lossy codec")
	}
	if res.Patched > 0 {
		fmt.Printf("Warning: %d malformed ALAC packet(s) detected in %s\n", res.Patched, trackPath)
	}
	return nil
}
