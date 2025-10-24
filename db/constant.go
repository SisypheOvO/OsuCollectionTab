package db

// OsuDB版本常量
const (
	// VersionWithoutEntrySize 不包含entry size字段的版本
	VersionWithoutEntrySize = 20191106

	// VersionWithByteAR AR等属性使用byte存储的版本
	VersionWithByteAR = 20140609

	// VersionWithExtraShort 包含额外short字段的版本
	VersionWithExtraShort = 20140609

	// VersionWithFloatStarRating 星级评分从Double改为Float的版本
	VersionWithFloatStarRating = 20250107
)

// 数据类型大小常量
const (
	SizeByte     = 1
	SizeShort    = 2
	SizeInt      = 4
	SizeLong     = 8
	SizeSingle   = 4
	SizeDouble   = 8
	SizeBoolean  = 1
	SizeDateTime = 8
)

// 字符串指示符
const (
	StringIndicatorEmpty  = 0x00
	StringIndicatorExists = 0x0b
)

// 文件大小限制
const (
	MaxStringLength = 1024 * 1024 // 1MB
	MaxTimingPoints = 10000       // 最大节奏点数量
)
