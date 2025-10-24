package db

import (
	"fmt"
	"encoding/binary"
	"os"
	"regexp"
	"bufio"
)

var (
	// MD5正则表达式 - 包级变量避免重复编译
	md5Regex = regexp.MustCompile(`[a-f0-9]{32}`)
)

// CollectionReader 读取collection.db文件
type CollectionReader struct {
	reader *bufio.Reader
}

// NewCollectionReader 创建新的collection.db读取器
func NewCollectionReader(path string) (*CollectionReader, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("打开collection.db失败: %w", err)
	}

	return &CollectionReader{
		reader: bufio.NewReader(file),
	}, nil
}

// ReadAllHashes 读取所有收藏夹中的谱面哈希
func (cr *CollectionReader) ReadAllHashes() (map[string]bool, error) {
	hashes := make(map[string]bool)

	// 版本号
	var version int32
	if err := binary.Read(cr.reader, binary.LittleEndian, &version); err != nil {
		return nil, fmt.Errorf("读取版本号失败: %w", err)
	}

	// 收藏夹数量
	var collectionCount int32
	if err := binary.Read(cr.reader, binary.LittleEndian, &collectionCount); err != nil {
		return nil, fmt.Errorf("读取收藏夹数量失败: %w", err)
	}

	if collectionCount == 0 {
		return hashes, nil
	}

	for i := int32(0); i < collectionCount; i++ {
		if err := cr.readCollection(hashes); err != nil {
			return nil, fmt.Errorf("读取第%d个收藏夹失败: %w", i+1, err)
		}
	}

	return hashes, nil
}

// readCollection 读取单个收藏夹的信息
func (cr *CollectionReader) readCollection(hashes map[string]bool) error {
	// 读取收藏夹名称
	if _, err := ParseString(cr.reader, true); err != nil {
		return fmt.Errorf("读取收藏夹名称失败: %w", err)
	}

	// 读取谱面数量
	var beatmapCount int32
	if err := binary.Read(cr.reader, binary.LittleEndian, &beatmapCount); err != nil {
		return fmt.Errorf("读取谱面数量失败: %w", err)
	}

	// 读取所有谱面哈希
	for j := int32(0); j < beatmapCount; j++ {
		hash, err := ParseString(cr.reader, false)
		if err != nil {
			return fmt.Errorf("读取第%d个哈希失败: %w", j+1, err)
		}

		// 验证并提取MD5哈希
		if matches := md5Regex.FindString(hash); matches != "" {
			hashes[matches] = true
		}
	}

	return nil
}

// ReadCollectionDB 读取collection.db文件(便捷函数)
func ReadCollectionDB(path string) (map[string]bool, error) {
	reader, err := NewCollectionReader(path)
	if err != nil {
		return nil, err
	}

	return reader.ReadAllHashes()
}
