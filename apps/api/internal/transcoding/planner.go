package transcoding

import (
	"fmt"
	"strings"
)

const (
	HLSModeRemux          = "remux_hls"
	HLSModeAudioTranscode = "audio_transcode_hls"
	HLSModeFullTranscode  = "full_transcode_hls"
)

type HLSPlan struct {
	Mode       string
	VideoCodec string
	AudioCodec string
	VideoCopy  bool
	AudioCopy  bool
}

func PlanHLS(info MediaInfo) (HLSPlan, error) {
	if info.Video == nil {
		return HLSPlan{}, fmt.Errorf("no video stream found")
	}

	videoCopy := canCopyVideoForHLS(*info.Video)
	audioCopy := info.Audio == nil || canCopyAudioForHLS(*info.Audio)

	plan := HLSPlan{
		VideoCodec: "libx264",
		AudioCodec: "aac",
		VideoCopy:  videoCopy,
		AudioCopy:  audioCopy,
	}

	if videoCopy {
		plan.VideoCodec = "copy"
	}
	if audioCopy {
		plan.AudioCodec = "copy"
	}

	switch {
	case videoCopy && audioCopy:
		plan.Mode = HLSModeRemux
	case videoCopy:
		plan.Mode = HLSModeAudioTranscode
	default:
		plan.Mode = HLSModeFullTranscode
		plan.VideoCopy = false
		plan.VideoCodec = "libx264"
		plan.AudioCopy = false
		plan.AudioCodec = "aac"
	}

	return plan, nil
}

func normalizeHLSPlan(plan HLSPlan) HLSPlan {
	switch strings.TrimSpace(plan.Mode) {
	case HLSModeRemux, HLSModeAudioTranscode, HLSModeFullTranscode:
		return plan
	default:
		return HLSPlan{
			Mode:       HLSModeFullTranscode,
			VideoCodec: "libx264",
			AudioCodec: "aac",
		}
	}
}

func canCopyVideoForHLS(stream MediaStream) bool {
	codec := normalizeMediaValue(stream.CodecName)
	if codec != "h264" && codec != "avc" && codec != "avc1" {
		return false
	}

	return normalizeMediaValue(stream.PixelFormat) == "yuv420p"
}

func canCopyAudioForHLS(stream MediaStream) bool {
	return normalizeMediaValue(stream.CodecName) == "aac"
}
