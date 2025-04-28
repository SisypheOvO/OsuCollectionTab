package db

import (
	"encoding/binary"
	"errors"
	"os"
	"regexp"
)

func ReadCollectionDB(path string) (map[string]bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if len(data) < 8 {
		return nil, errors.New("文件太小")
	}

	hashes := make(map[string]bool)
	offset := 4 // 跳过版本号

	collectionCount := binary.LittleEndian.Uint32(data[offset : offset+4])
	offset += 4

	if collectionCount == 0 {
		return hashes, nil
	}

	md5Regex := regexp.MustCompile(`[a-f0-9]{32}`)

	// 遍历 Collections
	for i := 0; i < int(collectionCount); i++ {
		if offset >= len(data) {
			break
		}

		// 读取收藏夹名称长度
		nameLen := int(data[offset])
		offset += 1

		// 跳过收藏夹名称
		if offset+nameLen > len(data) {
			break
		}
		offset += nameLen

		// 读取谱面数量
		if offset+4 > len(data) {
			break
		}
		beatmapCount := binary.LittleEndian.Uint32(data[offset : offset+4])
		offset += 4

		// 遍历单个 Collection 中所有哈希
		for j := 0; j < int(beatmapCount); j++ {
			if offset >= len(data) {
				break
			}

			// 读取哈希长度
			hashLen := int(data[offset])
			offset += 1

			if offset+hashLen > len(data) {
				break
			}

			// 提取可能的 MD5 哈希
			block := string(data[offset : offset+hashLen])
			matches := md5Regex.FindAllString(block, -1)
			for _, match := range matches {
				hashes[match] = true
			}

			offset += hashLen
		}
	}

	return hashes, nil
}
