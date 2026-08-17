package model

import "errors"

// 哨兵错误。调用方使用 errors.Is 沿错误链判定，不要比较错误文本。
var (
	// ErrUnknownBand 表示频段代码无法识别。
	ErrUnknownBand = errors.New("model: 未知频段")
	// ErrUnknownSatState 表示卫星状态代码无法识别。
	ErrUnknownSatState = errors.New("model: 未知卫星状态")
	// ErrUnknownStationKind 表示地面站类型代码无法识别。
	ErrUnknownStationKind = errors.New("model: 未知地面站类型")

	// ErrInvalidSatellite 表示卫星登记信息非法。
	ErrInvalidSatellite = errors.New("model: 卫星登记非法")
	// ErrInvalidStation 表示地面站登记信息非法。
	ErrInvalidStation = errors.New("model: 地面站登记非法")
	// ErrInvalidPass 表示可见弧信息非法。
	ErrInvalidPass = errors.New("model: 可见弧信息非法")
	// ErrInvalidGrant 表示频轨授权信息非法。
	ErrInvalidGrant = errors.New("model: 频轨授权非法")
	// ErrInvalidFrame 表示遥测帧非法。
	ErrInvalidFrame = errors.New("model: 遥测帧非法")

	// ErrSatelliteNotFound 表示卫星不存在。
	ErrSatelliteNotFound = errors.New("model: 卫星不存在")
	// ErrStationNotFound 表示地面站不存在。
	ErrStationNotFound = errors.New("model: 地面站不存在")
	// ErrPassNotFound 表示可见弧不存在。
	ErrPassNotFound = errors.New("model: 可见弧不存在")

	// ErrBandExhausted 表示该频段可用带宽已耗尽。
	ErrBandExhausted = errors.New("model: 频段带宽耗尽")
	// ErrBandOversubscribed 表示频段被超额分配。
	ErrBandOversubscribed = errors.New("model: 频段超额分配")
	// ErrWindowConflict 表示测控窗口冲突。
	ErrWindowConflict = errors.New("model: 测控窗口冲突")
	// ErrSatelliteNotOperational 表示卫星不在可测控状态。
	ErrSatelliteNotOperational = errors.New("model: 卫星不在可测控状态")

	// ErrFrameChecksum 表示遥测帧校验和不符。
	ErrFrameChecksum = errors.New("model: 遥测帧校验和不符")
	// ErrArchiveWrite 表示遥测归档写出失败。
	ErrArchiveWrite = errors.New("model: 遥测归档写出失败")
	// ErrArchiveIncomplete 表示遥测归档与入库明细不一致。
	ErrArchiveIncomplete = errors.New("model: 遥测归档与明细不一致")

	// ErrWindowSliceAliased 表示切分出的调度窗口段之间相互串扰。
	ErrWindowSliceAliased = errors.New("model: 调度窗口段相互串扰")

	// ErrUplinkUnavailable 表示上行通道不可用。
	ErrUplinkUnavailable = errors.New("model: 上行通道不可用")
	// ErrUplinkTimeout 表示上行通道超时。
	ErrUplinkTimeout = errors.New("model: 上行通道超时")
)
