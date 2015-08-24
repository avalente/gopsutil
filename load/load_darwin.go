// +build darwin

package load

// #include <sys/types.h>
// #include <sys/sysctl.h>
// #include <stdlib.h>
import "C"

import (
	"fmt"
	"unsafe"
)

func LoadAvg() (*LoadAvgStat, error) {
	name := C.CString("vm.loadavg")
	defer C.free(unsafe.Pointer(name))

	var size C.size_t
	var res C.int = C.sysctlbyname(name, nil, &size, nil, 0)

	if res != 0 {
		return nil, fmt.Errorf("errno %d", res)
	}

	buf := make([]byte, int(size))

	res = C.sysctlbyname(name, (unsafe.Pointer(&buf[0])), &size, nil, 0)
	if res != 0 {
		return nil, fmt.Errorf("errno %d", res)
	}

	var out C.struct_loadavg = *(*C.struct_loadavg)(unsafe.Pointer(&buf[0]))

	scale := float64(out.fscale)

	ret := &LoadAvgStat{
		Load1:  float64(out.ldavg[0]) / scale,
		Load5:  float64(out.ldavg[1]) / scale,
		Load15: float64(out.ldavg[2]) / scale,
	}

	return ret, nil
}
