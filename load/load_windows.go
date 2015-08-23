// +build windows

package load

import (
	common "github.com/avalente/gopsutil/common"
)

func LoadAvg() (*LoadAvgStat, error) {
	ret := LoadAvgStat{}

	return &ret, common.NotImplementedError
}
