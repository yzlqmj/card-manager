package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
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

// WriteCharaToPNG 将 'chara' 数据写入新的 PNG 文件
func WriteCharaToPNG(originalImagePath, outputPath, charaData string) error {
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

	// 复制 PNG 签名
	signature := make([]byte, 8)
	if _, err := io.ReadFull(inputFile, signature); err != nil {
		return err
	}
	if _, err := outputFile.Write(signature); err != nil {
		return err
	}

	// 重置文件指针以重新读取
	inputFile.Seek(8, 0)

	charaChunk := createTextChunk("chara", charaData)

	var iendChunk *chunk
	foundIEND := false

	for {
		ch, err := readChunk(inputFile)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		// 跳过旧的 chara 块
		if ch.Type == "tEXt" {
			if keyword := string(bytes.SplitN(ch.Data, []byte{0}, 2)[0]); keyword == "chara" {
				continue
			}
		}

		if ch.Type == "IEND" {
			iendChunk = ch
			foundIEND = true
			break
		}

		if err := writeChunk(outputFile, ch); err != nil {
			return fmt.Errorf("写入块 %s 失败: %w", ch.Type, err)
		}
	}

	if !foundIEND || iendChunk == nil {
		return errors.New("未在原始图像中找到 IEND 块")
	}

	// 在 IEND 之前写入新的 chara 块
	if err := writeChunk(outputFile, charaChunk); err != nil {
		return fmt.Errorf("写入 chara 块失败: %w", err)
	}

	// 写入 IEND 块
	if err := writeChunk(outputFile, iendChunk); err != nil {
		return fmt.Errorf("写入 IEND 块失败: %w", err)
	}

	return nil
}

// createTextChunk 创建一个 tEXt 块
func createTextChunk(keyword, text string) *chunk {
	data := append([]byte(keyword), 0)
	data = append(data, []byte(text)...)
	return &chunk{
		Type:   "tEXt",
		Data:   data,
		Length: uint32(len(data)),
	}
}

// readChunk 从 reader 中读取单个 PNG 块
func readChunk(reader io.Reader) (*chunk, error) {
	var length uint32
	if err := binary.Read(reader, binary.BigEndian, &length); err != nil {
		return nil, err
	}

	typeAndData := make([]byte, 4+length)
	if _, err := io.ReadFull(reader, typeAndData); err != nil {
		return nil, fmt.Errorf("读取块类型和数据失败: %w", err)
	}

	var crc uint32
	if err := binary.Read(reader, binary.BigEndian, &crc); err != nil {
		return nil, fmt.Errorf("读取块 CRC 失败: %w", err)
	}

	return &chunk{
		Length: length,
		Type:   string(typeAndData[:4]),
		Data:   typeAndData[4:],
		CRC:    crc,
	}, nil
}

// writeChunk 将单个 PNG 块写入 writer
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

	crc := crc32.NewIEEE()
	crc.Write(typeBytes)
	crc.Write(ch.Data)
	if err := binary.Write(writer, binary.BigEndian, crc.Sum32()); err != nil {
		return err
	}

	return nil
}

// chunk 代表一个 PNG 块
type chunk struct {
	Length uint32
	Type   string
	Data   []byte
	CRC    uint32
}
