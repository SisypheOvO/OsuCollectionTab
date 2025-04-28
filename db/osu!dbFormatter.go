// osudbFormatter.go
package db

import (
	"encoding/binary"
	"fmt"
	"io"
	"unicode/utf8"
)

// Different types's size in bytes
const (
	OSU_BYTE     = 1
	OSU_SHORT    = 2
	OSU_INT      = 4
	OSU_LONG     = 8
	OSU_SINGLE   = 4
	OSU_DOUBLE   = 8
	OSU_BOOLEAN  = 1
	OSU_DATETIME = 8
)

// ParseString 从文件对象读取OSU字符串
func ParseString(reader io.Reader, seek bool) (string, error) {
	indicator := make([]byte, 1)
	if _, err := reader.Read(indicator); err != nil {
		return "", fmt.Errorf("无法读取字符串标志: %w", err)
	}

	switch indicator[0] {
	case 0x00:
		return "", nil
	case 0x0b:
		length, err := ParseULEB128(reader)
		if err != nil {
			return "", fmt.Errorf("无法读取字符串长度: %w", err)
		}
		if length < 0 {
			return "", fmt.Errorf("无效的字符串长度: %d", length)
		}
		if length > 1024*1024 { // 示例：限制最大1MB
			return "", fmt.Errorf("字符串长度过长: %d", length)
		}

		if seek {
			// 如果需要跳过字符串内容，直接返回
			_, err := io.CopyN(io.Discard, reader, int64(length))
			if err != nil {
				return "", fmt.Errorf("跳过字符串内容失败: %w", err)
			}
			return "", nil
		}

		// 读取字符串内容
		strBytes := make([]byte, length)
		if _, err := io.ReadFull(reader, strBytes); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				return "", fmt.Errorf("字符串内容不完整，期望长度 %d: %w", length, err)
			}
			return "", fmt.Errorf("无法读取字符串内容: %w", err)
		}

		if !utf8.Valid(strBytes) {
			return "", fmt.Errorf("无效的UTF-8编码")
		}

		return string(strBytes), nil
	default:
		return "", fmt.Errorf("无效的字符串标志: 0x%02x", indicator[0])
	}
}

// ParseULEB128 读取无符号小端Base 128整数
func ParseULEB128(reader io.Reader) (uint64, error) {
	result := uint64(0)
	shift := uint(0)

	for {
		byteVal := make([]byte, 1)
		if _, err := reader.Read(byteVal); err != nil {
			return 0, fmt.Errorf("读取ULEB128失败: %w", err)
		}

		result |= uint64(byteVal[0]&0x7F) << shift

		if (byteVal[0] & 0x80) == 0 {
			break
		}

		shift += 7
	}

	return result, nil
}

// GetULEB128 将整数转换为ULEB128编码的字节
func GetULEB128(integer uint64) []byte {
	var result []byte

	for {
		b := integer & 0x7F
		integer >>= 7

		if integer != 0 {
			b |= 0x80
		}

		result = append(result, byte(b))

		if integer == 0 {
			break
		}
	}

	return result
}

// TimingPoint 表示节奏点
type TimingPoint struct {
	BPM         float64
	Offset      float64
	Uninherited bool
}

// IntDoublePair 表示整数-浮点数对
type IntDoublePair struct {
	Int    int32
	Double float64
}

type IntFloatPair struct {
	Int   int32
	Float float32
}

// ReadType 根据类型从文件中读取数据
func ReadType(typeName string, reader io.Reader) (interface{}, error) {
	switch typeName {
	case "Int":
		var val int32
		err := binary.Read(reader, binary.LittleEndian, &val)
		return val, err

	case "String":
		return ParseString(reader, false)

	case "Byte":
		var val byte
		err := binary.Read(reader, binary.LittleEndian, &val)
		return val, err

	case "Short":
		var val int16
		err := binary.Read(reader, binary.LittleEndian, &val)
		return val, err

	case "Long":
		var val int64
		err := binary.Read(reader, binary.LittleEndian, &val)
		return val, err

	case "Single": // 32位浮点数
		var val float32
		err := binary.Read(reader, binary.LittleEndian, &val)
		return val, err

	case "Double": // 64位浮点数
		var val float64
		err := binary.Read(reader, binary.LittleEndian, &val)
		return val, err

	case "Boolean":
		var val byte
		if err := binary.Read(reader, binary.LittleEndian, &val); err != nil {
			return false, err
		}
		return val != 0, nil

	case "IntDoublepair":
		// 读取标记字节
		var marker byte
		if err := binary.Read(reader, binary.LittleEndian, &marker); err != nil {
			return nil, err
		}

		// 读取整数
		var intVal int32
		if err := binary.Read(reader, binary.LittleEndian, &intVal); err != nil {
			return nil, err
		}

		// 读取另一个标记字节
		if err := binary.Read(reader, binary.LittleEndian, &marker); err != nil {
			return nil, err
		}

		// 读取浮点数
		var doubleVal float64
		if err := binary.Read(reader, binary.LittleEndian, &doubleVal); err != nil {
			return nil, err
		}

		return IntDoublePair{intVal, doubleVal}, nil

	case "IntFloatPair":
		// 读取标记字节
		var marker byte
		if err := binary.Read(reader, binary.LittleEndian, &marker); err != nil {
			return nil, err
		}

		// 读取整数
		var intVal int32
		if err := binary.Read(reader, binary.LittleEndian, &intVal); err != nil {
			return nil, err
		}

		// 读取另一个标记字节
		if err := binary.Read(reader, binary.LittleEndian, &marker); err != nil {
			return nil, err
		}

		// 读取浮点数
		var floatVal float32
		if err := binary.Read(reader, binary.LittleEndian, &floatVal); err != nil {
			return nil, err
		}

		return IntFloatPair{intVal, floatVal}, nil

	case "Timingpoint":
		// 读取BPM
		var bpm float64
		if err := binary.Read(reader, binary.LittleEndian, &bpm); err != nil {
			return nil, err
		}

		// 读取偏移
		var offset float64
		if err := binary.Read(reader, binary.LittleEndian, &offset); err != nil {
			return nil, err
		}

		// 读取是否为非继承标志
		var uninherited byte
		if err := binary.Read(reader, binary.LittleEndian, &uninherited); err != nil {
			return nil, err
		}

		return TimingPoint{bpm, offset, uninherited != 0}, nil

	case "DateTime":
		var val int64
		err := binary.Read(reader, binary.LittleEndian, &val)
		return val, err

	default:
		return nil, fmt.Errorf("未知数据类型: %s", typeName)
	}
}
