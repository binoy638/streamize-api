package transcoding

import "testing"

func TestParseFFprobeOutput(t *testing.T) {
	info, err := parseFFprobeOutput([]byte(`{
		"streams": [
			{"codec_type":"video","codec_name":"h264","profile":"High","width":1280,"height":720,"pix_fmt":"yuv420p","level":31},
			{"codec_type":"audio","codec_name":"ac3","sample_rate":"48000","channels":6,"bit_rate":"448000"},
			{"codec_type":"subtitle","codec_name":"subrip","tags":{"language":"eng","title":"English"}}
		],
		"format": {"format_name":"matroska,webm","duration":"1297.024000","bit_rate":"957840"}
	}`))
	if err != nil {
		t.Fatalf("parseFFprobeOutput returned error: %v", err)
	}

	if info.Container != "matroska,webm" || info.DurationSeconds != 1297.024 || info.BitRate != 957840 {
		t.Fatalf("unexpected format info: %+v", info)
	}
	if info.Video == nil || info.Video.CodecName != "h264" || info.Video.PixelFormat != "yuv420p" || info.Video.Width != 1280 || info.Video.Height != 720 {
		t.Fatalf("unexpected video stream: %+v", info.Video)
	}
	if info.Audio == nil || info.Audio.CodecName != "ac3" || info.Audio.SampleRate != 48000 || info.Audio.Channels != 6 || info.Audio.BitRate != 448000 {
		t.Fatalf("unexpected audio stream: %+v", info.Audio)
	}
	if len(info.Subtitles) != 1 || info.Subtitles[0].CodecName != "subrip" || info.Subtitles[0].Language != "eng" || info.Subtitles[0].Title != "English" {
		t.Fatalf("unexpected subtitle streams: %+v", info.Subtitles)
	}
}
