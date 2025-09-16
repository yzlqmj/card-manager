package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
	"os"
)

// getInternalCharNameFromPNG 从 PNG 文件中提取 'chara' 文本块
func getInternalCharNameFromPNG(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// PNG magic bytes
	header := make([]byte, 8)
	if _, err := io.ReadFull(file, header); err != nil {
		return "", err
	}
	if string(header) != "\x89PNG\r\n\x1a\n" {
		return "", errors.New("not a valid PNG file")
	}

	for {
		var length uint32
		if err := binary.Read(file, binary.BigEndian, &length); err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}

		chunkType := make([]byte, 4)
		if _, err := io.ReadFull(file, chunkType); err != nil {
			return "", err
		}

		chunkData := make([]byte, length)
		if _, err := io.ReadFull(file, chunkData); err != nil {
			return "", err
		}

		var crc uint32
		if err := binary.Read(file, binary.BigEndian, &crc); err != nil {
			return "", err
		}

		// 验证 CRC
		crcTable := crc32.MakeTable(crc32.IEEE)
		calculatedCrc := crc32.New(crcTable)
		calculatedCrc.Write(chunkType)
		calculatedCrc.Write(chunkData)
		if calculatedCrc.Sum32() != crc {
			// CRC 错误可以忽略，因为我们只关心数据
		}

		if string(chunkType) == "tEXt" {
			parts := bytes.SplitN(chunkData, []byte{0}, 2)
			if len(parts) == 2 && string(parts[0]) == "chara" {
				return string(parts[1]), nil
			}
		}

		if string(chunkType) == "IEND" {
			break
		}
	}

	return "", errors.New("'chara' text chunk not found")
}
