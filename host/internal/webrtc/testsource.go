package webrtc

import (
	"io"
	"log"
	"os"
	"time"

	pion "github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
	"github.com/pion/webrtc/v4/pkg/media/ivfreader"
)

func attachTestVideoTrack(pc *pion.PeerConnection, sourcePath string) error {
	track, err := pion.NewTrackLocalStaticSample(
		pion.RTPCodecCapability{MimeType: pion.MimeTypeVP8},
		"video",
		"share-app",
	)
	if err != nil {
		return err
	}

	if _, err := pc.AddTrack(track); err != nil {
		return err
	}

	go func() {
		for {
			if err := streamIVF(track, sourcePath); err != nil {
				log.Printf("test video stream error: %v", err)
				time.Sleep(2 * time.Second)
			}
		}
	}()

	return nil
}

func streamIVF(track *pion.TrackLocalStaticSample, sourcePath string) error {
	file, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer file.Close()

	reader, header, err := ivfreader.NewWith(file)
	if err != nil {
		return err
	}

	frameDuration := time.Second
	if header.TimebaseDenominator != 0 {
		frameDuration = time.Second * time.Duration(header.TimebaseNumerator) / time.Duration(header.TimebaseDenominator)
	}

	for {
		payload, _, err := reader.ParseNextFrame()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		if err := track.WriteSample(media.Sample{
			Data:     payload,
			Duration: frameDuration,
		}); err != nil {
			return err
		}
	}
}
