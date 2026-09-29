package engine

func estimate(meta Meta, format Format, bitrate int) int64 {
	if meta.Duration <= 0 {
		return 0
	}
	switch {
	case format == FLAC:
		bitrate = 900
	case format == WAV:
		bitrate = 1411
	case bitrate <= 0:
		return 0
	}
	return int64(float64(bitrate) * 1000 / 8 * meta.Duration)
}
