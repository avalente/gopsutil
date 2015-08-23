// +build darwin

package cpu

// #include <mach/mach.h>
// #include <mach/mach_error.h>
import "C"

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"unsafe"
)

var ClocksPerSec = float64(100)

func init() {
	out, err := exec.Command("/usr/bin/getconf", "CLK_TCK").Output()
	// ignore errors
	if err == nil {
		i, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
		if err == nil {
			ClocksPerSec = float64(i)
		}
	}
}

func fillCPUStats(ticks []uint32, cpu *CPUTimesStat) {
	cpu.User = float64(ticks[C.CPU_STATE_USER]) / ClocksPerSec
	cpu.System = float64(ticks[C.CPU_STATE_SYSTEM]) / ClocksPerSec
	cpu.Idle = float64(ticks[C.CPU_STATE_IDLE]) / ClocksPerSec
	cpu.Nice = float64(ticks[C.CPU_STATE_NICE]) / ClocksPerSec
}

// stolen from https://github.com/cloudfoundry/gosigar
func getCPUTotalTimes() (*CPUTimesStat, error) {
	var count C.mach_msg_type_number_t = C.HOST_CPU_LOAD_INFO_COUNT
	var cpuload C.host_cpu_load_info_data_t

	status := C.host_statistics(C.host_t(C.mach_host_self()),
		C.HOST_CPU_LOAD_INFO,
		C.host_info_t(unsafe.Pointer(&cpuload)),
		&count)

	if status != C.KERN_SUCCESS {
		return nil, fmt.Errorf("host_statistics error=%d", status)
	}

	cpu := CPUTimesStat{
		CPU: "total"}

	var cpu_ticks = make([]uint32, len(cpuload.cpu_ticks))
	for i := range cpuload.cpu_ticks {
		cpu_ticks[i] = uint32(cpuload.cpu_ticks[i])
	}

	fillCPUStats(cpu_ticks, &cpu)

	return &cpu, nil
}

// stolen from https://github.com/cloudfoundry/gosigar
func getCPUDetailedTimes() ([]CPUTimesStat, error) {
	var count C.mach_msg_type_number_t
	var cpuload *C.processor_cpu_load_info_data_t
	var ncpu C.natural_t

	status := C.host_processor_info(C.host_t(C.mach_host_self()),
		C.PROCESSOR_CPU_LOAD_INFO,
		&ncpu,
		(*C.processor_info_array_t)(unsafe.Pointer(&cpuload)),
		&count)

	if status != C.KERN_SUCCESS {
		return nil, fmt.Errorf("host_processor_info error=%d", status)
	}

	// jump through some cgo casting hoops and ensure we properly free
	// the memory that cpuload points to
	target := C.vm_map_t(C.mach_task_self_)
	address := C.vm_address_t(uintptr(unsafe.Pointer(cpuload)))
	defer C.vm_deallocate(target, address, C.vm_size_t(ncpu))

	// the body of struct processor_cpu_load_info
	// aka processor_cpu_load_info_data_t
	var cpu_ticks = make([]uint32, C.CPU_STATE_MAX)

	// copy the cpuload array to a []byte buffer
	// where we can binary.Read the data
	size := int(ncpu) * binary.Size(cpu_ticks)
	buf := C.GoBytes(unsafe.Pointer(cpuload), C.int(size))

	bbuf := bytes.NewBuffer(buf)

	var ret []CPUTimesStat = make([]CPUTimesStat, 0, ncpu+1)

	for i := 0; i < int(ncpu); i++ {
		err := binary.Read(bbuf, binary.LittleEndian, &cpu_ticks)
		if err != nil {
			return nil, err
		}

		cpu := CPUTimesStat{
			CPU: strconv.Itoa(i)}

		fillCPUStats(cpu_ticks, &cpu)

		ret = append(ret, cpu)
	}

	return ret, nil
}

func CPUTimes(percpu bool) ([]CPUTimesStat, error) {
	if percpu {
		return getCPUDetailedTimes()
	} else {
		cpu, err := getCPUTotalTimes()
		return []CPUTimesStat{*cpu}, err
	}
}

// Returns only one CPUInfoStat on FreeBSD
func CPUInfo() ([]CPUInfoStat, error) {
	var ret []CPUInfoStat

	out, err := exec.Command("/usr/sbin/sysctl", "machdep.cpu").Output()
	if err != nil {
		return ret, err
	}

	c := CPUInfoStat{}
	for _, line := range strings.Split(string(out), "\n") {
		values := strings.Fields(line)
		if len(values) < 1 {
			continue
		}

		t, err := strconv.ParseInt(values[1], 10, 64)
		// err is not checked here because some value is string.
		if strings.HasPrefix(line, "machdep.cpu.brand_string") {
			c.ModelName = strings.Join(values[1:], " ")
		} else if strings.HasPrefix(line, "machdep.cpu.family") {
			c.Family = values[1]
		} else if strings.HasPrefix(line, "machdep.cpu.model") {
			c.Model = values[1]
		} else if strings.HasPrefix(line, "machdep.cpu.stepping") {
			if err != nil {
				return ret, err
			}
			c.Stepping = int32(t)
		} else if strings.HasPrefix(line, "machdep.cpu.features") {
			for _, v := range values[1:] {
				c.Flags = append(c.Flags, strings.ToLower(v))
			}
		} else if strings.HasPrefix(line, "machdep.cpu.leaf7_features") {
			for _, v := range values[1:] {
				c.Flags = append(c.Flags, strings.ToLower(v))
			}
		} else if strings.HasPrefix(line, "machdep.cpu.extfeatures") {
			for _, v := range values[1:] {
				c.Flags = append(c.Flags, strings.ToLower(v))
			}
		} else if strings.HasPrefix(line, "machdep.cpu.core_count") {
			if err != nil {
				return ret, err
			}
			c.Cores = int32(t)
		} else if strings.HasPrefix(line, "machdep.cpu.cache.size") {
			if err != nil {
				return ret, err
			}
			c.CacheSize = int32(t)
		} else if strings.HasPrefix(line, "machdep.cpu.vendor") {
			c.VendorID = values[1]
		}

		// TODO:
		// c.Mhz = mustParseFloat64(values[1])
	}

	return append(ret, c), nil
}
