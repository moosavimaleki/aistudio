package browserinterface

import (
	"os"
	"strconv"
	"strings"
)

const browserMemoryReserve = uint64(512 * 1024 * 1024)
const browserLaunchMemory = uint64(384 * 1024 * 1024)

// Use both the host budget and the container budget. Host MemAvailable alone
// can look healthy while the container is already at its cgroup limit.
func browserMemoryAvailable() uint64 {
	return memoryAvailable(readMemoryFile)
}

func readMemoryFile(path string) string {
	value, _ := os.ReadFile(path)
	return string(value)
}

func memoryAvailable(read func(string) string) uint64 {
	available := uint64(0)
	for _, line := range strings.Split(read("/proc/meminfo"), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "MemAvailable:" {
			value, err := strconv.ParseUint(fields[1], 10, 64)
			if err == nil {
				available = value * 1024
			}
		}
	}
	for _, pair := range [][2]string{
		{"/sys/fs/cgroup/memory.max", "/sys/fs/cgroup/memory.current"},
		{"/sys/fs/cgroup/memory/memory.limit_in_bytes", "/sys/fs/cgroup/memory/memory.usage_in_bytes"},
	} {
		limit, err := strconv.ParseUint(strings.TrimSpace(read(pair[0])), 10, 64)
		if err != nil {
			continue
		}
		used, err := strconv.ParseUint(strings.TrimSpace(read(pair[1])), 10, 64)
		if err != nil {
			continue
		}
		free := uint64(0)
		if limit > used {
			free = limit - used
		}
		if free < available {
			available = free
		}
	}
	return available
}
