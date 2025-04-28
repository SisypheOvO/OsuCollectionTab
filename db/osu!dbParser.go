// osudbParser.go
package db

import (
	"fmt"
	"io"
	"log"
	"os"
)

// Difficulty2 表示一个谱面难度
type Difficulty2 struct {
	Path         string
	Name         string
	Artist       string
	Mapper       string
	Difficulty   string
	AR           interface{} // float32或byte
	CS           interface{} // float32或byte
	HP           interface{} // float32或byte
	OD           interface{} // float32或byte
	Hash         string
	FromAPI      bool
	APIBeatmapID int32
	BeatmapID    int32
	BeatmapsetID int32
}

// Song 表示一个歌曲，包含多个难度的谱面
type Song struct {
	Difficulties []Difficulty2
}

// Songs 表示一个歌曲集合
type Songs struct {
	List []Song
}

// AddSong 添加歌曲到集合
func (s *Songs) AddSong(song Song) {
	s.List = append(s.List, song)
}

// ParseBeatmap 解析单个谱面
func ParseBeatmap(reader io.Reader, version int32) (*Difficulty2, error) {
	// 如果版本小于 20191106 ，需要读取一个整数
	if version < 20191106 {
		_, err := ReadType("Int", reader)
		if err != nil {
			return nil, fmt.Errorf("读取版本特定整数失败: %w", err)
		}
	}

	// 读取谱面的基本数据
	types := []string{
		"String", "String", "String", "String", // artist, artist_u, song, song_u
		"String", "String", "String", "String", "String", // creator, difficulty, audio_file, md5, osu_file
		"Byte",                    // ranked_status
		"Short", "Short", "Short", // num_hitcircles, num_sliders, num_spinners
		"Long", // last_modified
	}

	var data []interface{}
	for _, t := range types {
		val, err := ReadType(t, reader)
		if err != nil {
			return nil, fmt.Errorf("读取谱面数据失败: %w", err)
		}
		data = append(data, val)
	}

	artist := data[0].(string)
	song := data[2].(string)
	creator := data[4].(string)
	difficulty := data[5].(string)
	md5 := data[7].(string)
	osu_file := data[8].(string)

	// 读取AR, CS, HP, OD
	var ar, cs, hp, od interface{}
	if version < 20140609 {
		ar, _ = ReadType("Byte", reader)
		cs, _ = ReadType("Byte", reader)
		hp, _ = ReadType("Byte", reader)
		od, _ = ReadType("Byte", reader)
	} else {
		ar, _ = ReadType("Single", reader)
		cs, _ = ReadType("Single", reader)
		hp, _ = ReadType("Single", reader)
		od, _ = ReadType("Single", reader)
	}

	// 读取滑条速度
	sv, err := ReadType("Double", reader)
	if err != nil {
		return nil, fmt.Errorf("读取滑条速度失败: %w", err)
	}

	fmt.Printf("滑条速度: %v\n", sv)

	// 读取各模式的星级评分
	star_ratings := make([][]IntFloatPair, 4)
	for i := 0; i < 4; i++ {
		numPairs, err := ReadType("Int", reader)
		fmt.Printf("星级评分数量: %v\n", numPairs)
		if err != nil {
			return nil, fmt.Errorf("读取星级评分数量失败: %w", err)
		}

		pairs := make([]IntFloatPair, numPairs.(int32))
		for j := 0; j < int(numPairs.(int32)); j++ {
			pair, err := ReadType("IntFloatPair", reader)
			if err != nil {
				return nil, fmt.Errorf("读取星级评分对失败: %w", err)
			}
			pairs[j] = pair.(IntFloatPair)
		}
		star_ratings[i] = pairs
	}

	// 读取 Drain Time、总时间和预览时间
	_, err = ReadType("Int", reader)
	if err != nil {
		return nil, err
	}

	_, err = ReadType("Int", reader)
	if err != nil {
		return nil, err
	}

	_, err = ReadType("Int", reader)
	if err != nil {
		return nil, err
	}

	// 读取节奏点
	num_timingpoints, err := ReadType("Int", reader)
	if err != nil {
		return nil, err
	}

	timingpoints := make([]TimingPoint, num_timingpoints.(int32))
	for i := 0; i < int(num_timingpoints.(int32)); i++ {
		tp, err := ReadType("Timingpoint", reader)
		if err != nil {
			return nil, fmt.Errorf("读取节奏点失败: %w", err)
		}
		timingpoints[i] = tp.(TimingPoint)
	}

	// 读取更多谱面数据
	more_types := []string{
		"Int", "Int", "Int", // beatmap_id, beatmap_set_id, thread_id
		"Byte", "Byte", "Byte", "Byte", // grade_standard, grade_taiko, grade_ctb, grade_mania
		"Short", "Single", "Byte", // local_offset, stack_leniency, gameplay_mode
		"String", "String", // source, tags
		"Short", "String", "Boolean", "Long", "Boolean", "String", "Long", // online_offset, font, unplayed, last_played, is_osz2, beatmap_folder, last_checked
		"Boolean", "Boolean", "Boolean", "Boolean", "Boolean", // ignore_sounds, ignore_skin, disable_storyboard, disable_video, visual_override
	}

	more_data := make([]interface{}, len(more_types))
	for i, t := range more_types {
		more_data[i], err = ReadType(t, reader)
		if err != nil {
			return nil, fmt.Errorf("读取额外谱面数据失败: %w", err)
		}
	}

	beatmap_id := more_data[0].(int32)
	beatmap_set_id := more_data[1].(int32)

	// 如果版本小于20140609，需要读取一个额外的short
	if version < 20140609 {
		_, err := ReadType("Short", reader)
		if err != nil {
			return nil, err
		}
	}

	// 读取最后的修改时间和mania卷轴速度
	_, err = ReadType("Int", reader)
	if err != nil {
		return nil, err
	}

	_, err = ReadType("Byte", reader)
	if err != nil {
		return nil, err
	}

	// 创建并返回难度对象
	beatmap := &Difficulty2{
		Path:         osu_file,
		Name:         song,
		Artist:       artist,
		Mapper:       creator,
		Difficulty:   difficulty,
		AR:           ar,
		CS:           cs,
		HP:           hp,
		OD:           od,
		Hash:         md5,
		FromAPI:      false,
		APIBeatmapID: beatmap_id,
		BeatmapID:    beatmap_id,
		BeatmapsetID: beatmap_set_id,
	}

	log.Printf("加载谱面 %d: %s - %s [%s] by %s",
		beatmap.BeatmapID, beatmap.Artist,
		beatmap.Name, beatmap.Difficulty, beatmap.Mapper)

	return beatmap, nil
}

// LoadOsuDB 加载osu!.db文件
func LoadOsuDB(path string) (*Songs, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	songs := &Songs{}

	// 读取文件头数据
	types := []string{"Int", "Int", "Boolean", "DateTime", "String", "Int"}
	var header_data []interface{}

	for _, t := range types {
		val, err := ReadType(t, file)
		fmt.Printf("%s值: %v\n", t, val)
		if err != nil {
			return nil, fmt.Errorf("读取文件头数据失败: %w", err)
		}
		header_data = append(header_data, val)
	}

	version := header_data[0].(int32)
	num_maps := header_data[5].(int32)

	log.Printf("osu!DB 版本 %d，包含 %d 张谱面", version, num_maps)

	// 读取所有谱面
	beatmaps := make([]Difficulty2, 0, num_maps)
	for i := 0; i < int(num_maps); i++ {
		beatmap, err := ParseBeatmap(file, version)
		if err != nil {
			return nil, fmt.Errorf("解析谱面失败: %w", err)
		}
		beatmaps = append(beatmaps, *beatmap)
	}

	// 按照谱面集ID将谱面分组
	mapsets := make(map[int32][]Difficulty2)
	for _, beatmap := range beatmaps {
		mapsets[beatmap.BeatmapsetID] = append(mapsets[beatmap.BeatmapsetID], beatmap)
	}

	// 将分组后的谱面转换为歌曲集合
	for _, mapset := range mapsets {
		song := Song{
			Difficulties: mapset,
		}
		songs.AddSong(song)
	}

	return songs, nil
}

func ParseBeatmapForHash(reader io.Reader, version int32) (*Difficulty2, error) {
	for i := 0; i < 7; i++ {
		_, err := ParseString(reader, true)
		if err != nil {
			return nil, fmt.Errorf("读取谱面数据失败: %w", err)
		}
	}

	val, err := ReadType("String", reader)
	if err != nil {
		return nil, fmt.Errorf("读取谱面数据失败: %w", err)
	}
	md5 := val.(string)

	_, err = ParseString(reader, true)
	if err != nil {
		return nil, fmt.Errorf("读取谱面数据失败: %w", err)
	}

	_, err = io.CopyN(io.Discard, reader, 15)

	if version < 20140609 {
		_, err = io.CopyN(io.Discard, reader, 4)
	} else {
		_, err = io.CopyN(io.Discard, reader, 16)
	}

	_, err = io.CopyN(io.Discard, reader, 8)

	for i := 0; i < 4; i++ {
		numPairs, err := ReadType("Int", reader)
		if err != nil {
			return nil, fmt.Errorf("读取星级评分数量失败: %w", err)
		}

		for j := 0; j < int(numPairs.(int32)); j++ {
			_, err := ReadType("IntFloatPair", reader)
			if err != nil {
				return nil, fmt.Errorf("读取星级评分对失败: %w", err)
			}
		}
	}

	_, err = io.CopyN(io.Discard, reader, 12)

	// 读取节奏点
	num_timingpoints, err := ReadType("Int", reader)
	if err != nil {
		return nil, err
	}

	for i := 0; i < int(num_timingpoints.(int32)); i++ {
		_, err = io.CopyN(io.Discard, reader, 17)
		if err != nil {
			return nil, fmt.Errorf("读取节奏点失败: %w", err)
		}
	}

	_, err = io.CopyN(io.Discard, reader, 23)

	// 读取更多谱面数据
	more_types := []string{
		"String", "String", // source, tags
		"Short", "String", "Boolean", "Long", "Boolean", "String",
	}

	more_data := make([]interface{}, len(more_types))
	for i, t := range more_types {
		more_data[i], err = ReadType(t, reader)
		if err != nil {
			return nil, fmt.Errorf("读取额外谱面数据失败: %w", err)
		}
	}

	_, err = io.CopyN(io.Discard, reader, 13)

	// 如果版本小于20140609，需要读取一个额外的short
	if version < 20140609 {
		_, err = io.CopyN(io.Discard, reader, 2)
	}

	_, err = io.CopyN(io.Discard, reader, 5)

	// 创建并返回难度对象
	beatmap := &Difficulty2{
		Hash: md5,
	}

	return beatmap, nil
}

func LoadOsuDBForHash(path string) ([]Difficulty2, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	val, err := ReadType("Int", file)
	if err != nil {
		return nil, fmt.Errorf("读取osuDB版本失败: %w", err)
	}
	version := val.(int32)

	_, err = file.Seek(13, io.SeekCurrent)

	_, err = ReadType("String", file)
	if err != nil {
		return nil, fmt.Errorf("读取文件头数据失败: %w", err)
	}

	val, err = ReadType("Int", file)
	if err != nil {
		return nil, fmt.Errorf("读取文件头数据失败: %w", err)
	}
	num_maps := val.(int32)

	// 读取所有谱面
	beatmaps := make([]Difficulty2, 0, num_maps)
	for i := 0; i < int(num_maps); i++ {
		beatmap, err := ParseBeatmapForHash(file, version)
		if err != nil {
			return nil, fmt.Errorf("解析谱面失败: %w", err)
		}
		beatmaps = append(beatmaps, *beatmap)
	}

	return beatmaps, nil
}
