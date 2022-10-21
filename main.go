package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/drgolem/go-flac/flac"
	"github.com/drgolem/go-mpg123/mpg123"
	"github.com/drgolem/go-portaudio/portaudio"
	"github.com/drgolem/simpleFilePlayer/decoders"
)

type FileFormatType string

const (
	FileFormat_MP3  FileFormatType = ".mp3"
	FileFormat_OGG  FileFormatType = ".ogg"
	FileFormat_FLAC FileFormatType = ".flac"
)

type musicDecoder interface {
	GetFormat() (int, int, int)
	DecodeSamples(samples int, audio []byte) (int, error)

	Open(fileName string) error
	Close() error
	//io.Seeker
}

func main() {
	fmt.Println("test music player")

	filePtr := flag.String("in", "", "music file to play")

	flag.Parse()

	if *filePtr == "" {
		flag.PrintDefaults()
		return
	}
	fileName := *filePtr
	ext := filepath.Ext(fileName)
	fileFormat := FileFormatType(ext)
	switch fileFormat {
	case FileFormat_MP3, FileFormat_OGG, FileFormat_FLAC:
	default:
		fmt.Printf("Unsupported file format: %s\n", ext)
		flag.PrintDefaults()
		return
	}

	fmt.Printf("Playing: %s\n", fileName)
	fmt.Printf("Press Ctrl-C to stop.\n")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, os.Interrupt)

	var decoder musicDecoder

	switch fileFormat {
	case FileFormat_MP3:
		mp3Decoder, err := mpg123.NewDecoder("")
		if err != nil {
			fmt.Printf("ERR: %v\n", err)
			return
		}
		defer mp3Decoder.Delete()

		fmt.Printf("Decoder: %s\n", mp3Decoder.CurrentDecoder())
		decoder = mp3Decoder
	case FileFormat_OGG:
		streamType, err := decoders.GetOggFileStreamType(fileName)
		if err != nil {
			fmt.Printf("ERR: %v\n", err)
			return
		}
		fmt.Printf("file %s, stream type: %v\n", fileName, streamType)
		if streamType == decoders.StreamType_Vorbis {
			vorbisDecoder, err := decoders.NewOggVorbisDecoder()
			if err != nil {
				fmt.Printf("ERR: %v\n", err)
				return
			}
			decoder = vorbisDecoder
		} else if streamType == decoders.StreamType_Opus {
			//opusDecoder, err := decoders.NewOggOpusDecoder()
			opusDecoder, err := decoders.NewOggOpusFileDecoder()
			if err != nil {
				fmt.Printf("ERR: %v\n", err)
				return
			}
			decoder = opusDecoder
		}
	case FileFormat_FLAC:
		flacDecoder, err := flac.NewFlacFrameDecoder(16)
		if err != nil {
			fmt.Printf("ERR: %v\n", err)
			return
		}
		decoder = flacDecoder
	default:
		fmt.Printf("Unsupported file format: %s\n", ext)
		flag.PrintDefaults()
		return
	}

	if decoder == nil {
		fmt.Printf("unknown decoder\n")
		return
	}
	err := decoder.Open(fileName)
	if err != nil {
		fmt.Printf("ERR: %v\n", err)
		return
	}
	defer decoder.Close()

	// get audio format information
	rate, channels, _ := decoder.GetFormat()

	fmt.Printf("Encoding: Signed 16bit\n")
	fmt.Printf("Sample Rate: %d\n", rate)
	fmt.Printf("Channels: %d\n", channels)

	deviceIdx := 1
	sampleformat := portaudio.SampleFmtInt16

	portaudio.Initialize()
	defer portaudio.Terminate()
	outStreamParams := portaudio.PaStreamParameters{
		DeviceIndex:  deviceIdx,
		ChannelCount: channels,
		SampleFormat: sampleformat,
	}
	stream, err := portaudio.NewStream(outStreamParams, float32(rate))
	if err != nil {
		fmt.Printf("ERR: %v\n", err)
		return
	}

	const framesPerBuffer = 2048
	const audioBufSize = 4 * 2 * framesPerBuffer

	err = stream.Open(framesPerBuffer)
	if err != nil {
		fmt.Printf("ERR: %v\n", err)
		return
	}
	defer stream.Close()

	err = stream.StartStream()
	if err != nil {
		fmt.Printf("ERR: %v\n", err)
		return
	}
	defer stream.StopStream()

	for play := true; play; {
		audio := make([]byte, audioBufSize)
		nSamples, err := decoder.DecodeSamples(framesPerBuffer, audio)
		if nSamples == 0 {
			break
		}
		if err != nil {
			fmt.Printf("ERR: %v\n", err)
			return
		}

		err = stream.Write(nSamples, audio)
		if err != nil {
			fmt.Printf("ERR: %v\n", err)
			return
		}
		select {
		case <-sig:
			play = false
		default:
		}
	}
}
