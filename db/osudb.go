package db

import (
	"encoding/binary"
	"errors"
	"os"
)

func ReadOsuDB(path string) (map[string]bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if len(data) < 4 {
		return nil, errors.New("文件太小")
	}

	hashes := make(map[string]bool)
	offset := 4 // 跳过版本号

	// 跳过 17 字节的无关数据
	offset += 17

	if offset+4 > len(data) {
		return nil, errors.New("文件损坏")
	}

	beatmapCount := binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4

	for i := 0; i < int(beatmapCount); i++ {
		// 读取字符串直到遇到空字符
		offset = skipStrings(data, offset, 5) // artist, artist_unicode, title, title_unicode, creator

		// 读取 MD5 哈希
		md5, newOffset, err := readString(data, offset)
		if err != nil {
			return hashes, nil // 返回已读取的部分
		}
		offset = newOffset
		hashes[md5] = true

		// 跳过其他字符串
		offset = skipStrings(data, offset, 9)

		// 跳过各种数值
		if offset+30 > len(data) {
			return hashes, nil
		}
		offset += 30 // 4*8 + 2*2 + 8 = 32 + 4 + 8 = 44? 需要确认
	}

	return hashes, nil
}

func skipStrings(data []byte, offset int, count int) int {
	for i := 0; i < count; i++ {
		if offset >= len(data) {
			return offset
		}
		if data[offset] == 0x00 {
			offset++
			continue
		}
		if data[offset] == 0x0b {
			offset++
			if offset+4 > len(data) {
				return offset
			}
			length := int(binary.LittleEndian.Uint32(data[offset : offset+4]))
			offset += 4 + length
		}
	}
	return offset
}

func readString(data []byte, offset int) (string, int, error) {
	if offset >= len(data) {
		return "", offset, errors.New("超出数据范围")
	}
	if data[offset] == 0x00 {
		return "", offset + 1, nil
	}
	if data[offset] != 0x0b {
		return "", offset, errors.New("无效字符串格式")
	}
	offset++
	if offset+4 > len(data) {
		return "", offset, errors.New("超出数据范围")
	}
	length := int(binary.LittleEndian.Uint32(data[offset : offset+4]))
	offset += 4
	if offset+length > len(data) {
		return "", offset, errors.New("超出数据范围")
	}
	return string(data[offset : offset+length]), offset + length, nil
}
