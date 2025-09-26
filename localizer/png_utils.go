package localizer

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
)

// tEXtChunk represents a tEXt chunk in a PNG file.
type tEXtChunk struct {
	Keyword string
	Text    string
}

// readChunks reads all chunks from a PNG file.
func readChunks(reader io.Reader) ([]*chunk, error) {
	// PNG signature
	signature := make([]byte, 8)
	if _, err := io.ReadFull(reader, signature); err != nil {
		return nil, fmt.Errorf("could not read PNG signature: %w", err)
	}
	if string(signature) != "\x89PNG\r\n\x1a\n" {
		return nil, errors.New("invalid PNG signature")
	}

	var chunks []*chunk
	for {
		ch, err := readChunk(reader)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		chunks = append(chunks, ch)
		if ch.Type == "IEND" {
			break
		}
	}
	return chunks, nil
}

// chunk represents a PNG chunk.
type chunk struct {
	Length uint32
	Type   string
	Data   []byte
	CRC    uint32
}

// readChunk reads a single PNG chunk.
func readChunk(reader io.Reader) (*chunk, error) {
	var length uint32
	if err := binary.Read(reader, binary.BigEndian, &length); err != nil {
		return nil, err
	}

	typeAndData := make([]byte, 4+length)
	if _, err := io.ReadFull(reader, typeAndData); err != nil {
		return nil, fmt.Errorf("failed to read chunk type and data: %w", err)
	}

	var crc uint32
	if err := binary.Read(reader, binary.BigEndian, &crc); err != nil {
		return nil, fmt.Errorf("failed to read chunk CRC: %w", err)
	}

	chunkType := string(typeAndData[:4])
	data := typeAndData[4:]

	// Verify CRC
	crcHasher := crc32.NewIEEE()
	crcHasher.Write(typeAndData[:4]) // Write chunk type
	crcHasher.Write(data)            // Write chunk data
	if crcHasher.Sum32() != crc {
		return nil, fmt.Errorf("CRC mismatch in chunk %s", chunkType)
	}

	return &chunk{
		Length: length,
		Type:   chunkType,
		Data:   data,
		CRC:    crc,
	}, nil
}

// writeChunk writes a single PNG chunk to the writer.
func writeChunk(writer io.Writer, ch *chunk) error {
	if err := binary.Write(writer, binary.BigEndian, ch.Length); err != nil {
		return err
	}

	typeBytes := []byte(ch.Type)
	if _, err := writer.Write(typeBytes); err != nil {
		return err
	}
	if _, err := writer.Write(ch.Data); err != nil {
		return err
	}

	// Calculate and write CRC
	crc := crc32.NewIEEE()
	crc.Write(typeBytes)
	crc.Write(ch.Data)
	if err := binary.Write(writer, binary.BigEndian, crc.Sum32()); err != nil {
		return err
	}

	return nil
}

// decodeTextChunk decodes a tEXt chunk's data.
func decodeTextChunk(data []byte) (*tEXtChunk, error) {
	parts := bytes.SplitN(data, []byte{0}, 2)
	if len(parts) != 2 {
		return nil, errors.New("invalid tEXt chunk format")
	}
	return &tEXtChunk{
		Keyword: string(parts[0]),
		Text:    string(parts[1]),
	}, nil
}

// encodeTextChunk encodes a tEXt chunk's data.
func encodeTextChunk(keyword, text string) []byte {
	return append(append([]byte(keyword), 0), []byte(text)...)
}

// GetCharacterData reads character data from a PNG file.
// It prioritizes 'ccv3' over 'chara'.
func GetCharacterData(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	chunks, err := readChunks(reader)
	if err != nil {
		return "", fmt.Errorf("failed to read png chunks: %w", err)
	}

	var charaData, ccv3Data string

	for _, ch := range chunks {
		if ch.Type == "tEXt" {
			textChunk, err := decodeTextChunk(ch.Data)
			if err != nil {
				// Ignore invalid tEXt chunks
				continue
			}
			if textChunk.Keyword == "chara" {
				charaData = textChunk.Text
			} else if textChunk.Keyword == "ccv3" {
				ccv3Data = textChunk.Text
			}
		}
	}

	if ccv3Data != "" {
		return ccv3Data, nil
	}
	if charaData != "" {
		return charaData, nil
	}

	return "", errors.New("no character data found in PNG")
}

// WriteCharacterData writes character data to a new PNG file.
func WriteCharacterData(originalImagePath, outputPath, charaV2, charaV3 string) error {
	inputFile, err := os.Open(originalImagePath)
	if err != nil {
		return err
	}
	defer inputFile.Close()

	outputFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer outputFile.Close()

	// Write PNG signature
	if _, err := outputFile.WriteString("\x89PNG\r\n\x1a\n"); err != nil {
		return err
	}

	reader := bufio.NewReader(inputFile)
	// Skip signature
	if _, err := reader.Discard(8); err != nil {
		return err
	}

	charaV2Chunk := &chunk{
		Type: "tEXt",
		Data: encodeTextChunk("chara", charaV2),
	}
	charaV2Chunk.Length = uint32(len(charaV2Chunk.Data))

	charaV3Chunk := &chunk{
		Type: "tEXt",
		Data: encodeTextChunk("ccv3", charaV3),
	}
	charaV3Chunk.Length = uint32(len(charaV3Chunk.Data))

	var iendChunk *chunk
	foundIEND := false

	for {
		ch, err := readChunk(reader)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		// Skip existing chara/ccv3 chunks
		if ch.Type == "tEXt" {
			keyword := string(bytes.SplitN(ch.Data, []byte{0}, 2)[0])
			if keyword == "chara" || keyword == "ccv3" {
				continue
			}
		}

		if ch.Type == "IEND" {
			iendChunk = ch
			foundIEND = true
			break
		}

		if err := writeChunk(outputFile, ch); err != nil {
			return fmt.Errorf("failed to write chunk %s: %w", ch.Type, err)
		}
	}

	if !foundIEND || iendChunk == nil {
		return errors.New("IEND chunk not found in original image")
	}

	// Write our new chunks before IEND
	if err := writeChunk(outputFile, charaV2Chunk); err != nil {
		return fmt.Errorf("failed to write chara v2 chunk: %w", err)
	}
	if err := writeChunk(outputFile, charaV3Chunk); err != nil {
		return fmt.Errorf("failed to write chara v3 chunk: %w", err)
	}

	// Write IEND chunk
	if err := writeChunk(outputFile, iendChunk); err != nil {
		return fmt.Errorf("failed to write IEND chunk: %w", err)
	}

	return nil
}
